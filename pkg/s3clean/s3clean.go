package s3clean

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
)

type S3Cleaner interface {
	SyncS3Bucket() error
	WipeS3Bucket() error
}
type S3API interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	DeleteObjects(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}
type s3clean struct {
	ctx context.Context
	cfg models.Config
	svc S3API
	l   *zerolog.Logger
}

func New(cfg models.Config, svc S3API, l *zerolog.Logger) S3Cleaner {
	return NewWithContext(context.Background(), cfg, svc, l)
}
func NewWithContext(ctx context.Context, cfg models.Config, svc S3API, l *zerolog.Logger) S3Cleaner {
	if l == nil {
		logger := zerolog.Nop()
		l = &logger
	}
	return &s3clean{ctx: ctx, cfg: cfg, svc: svc, l: l}
}

// WipeS3Bucket deletes current objects. Versions and delete markers are retained.
func (s *s3clean) WipeS3Bucket() error {
	if err := s.cfg.Validate(); err != nil {
		return err
	}
	return s.pages("", func(objects []types.Object) error {
		ids := make([]types.ObjectIdentifier, 0, len(objects))
		for _, object := range objects {
			if object.Key == nil {
				return fmt.Errorf("S3 returned an object without a key")
			}
			if s.cfg.DryRun {
				s.l.Info().Str("key", *object.Key).Msg("would delete object")
				continue
			}
			ids = append(ids, types.ObjectIdentifier{Key: object.Key})
		}
		if len(ids) == 0 {
			return nil
		}
		result, err := s.svc.DeleteObjects(s.ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.cfg.AWS.S3Bucket), Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return fmt.Errorf("delete objects: %w", err)
		}
		if result == nil {
			return fmt.Errorf("delete objects returned an empty response")
		}
		var failures []error
		for _, failure := range result.Errors {
			failures = append(failures, fmt.Errorf("delete %q: %s: %s", aws.ToString(failure.Key), aws.ToString(failure.Code), aws.ToString(failure.Message)))
		}
		return errors.Join(failures...)
	})
}

// SyncS3Bucket only examines canonical keys below the configured local roots.
// OpenRoot prevents a remote key or local symlink from escaping those roots.
func (s *s3clean) SyncS3Bucket() error {
	if err := s.cfg.Validate(); err != nil {
		return err
	}
	dirs, err := models.BackupRoots(s.cfg.AWS.BackupDirectories)
	if err != nil {
		return err
	}
	roots := make([]*os.Root, 0, len(dirs))
	defer func() {
		for _, root := range roots {
			root.Close()
		}
	}()
	// Validate every source before allowing any deletes, including unreadable subtrees.
	for _, dir := range dirs {
		root, err := os.OpenRoot(dir)
		if err != nil {
			return err
		}
		roots = append(roots, root)
		if err := fs.WalkDir(root.FS(), ".", func(_ string, _ fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			return s.ctx.Err()
		}); err != nil {
			return fmt.Errorf("inspect backup root %q: %w", dir, err)
		}
	}
	for i, dir := range dirs {
		root := roots[i]
		prefix := strings.TrimSuffix(filepath.ToSlash(dir), "/") + "/"
		err := s.pages(prefix, func(objects []types.Object) error {
			for _, object := range objects {
				if err := s.ctx.Err(); err != nil {
					return err
				}
				if object.Key == nil {
					return fmt.Errorf("S3 returned an object without a key")
				}
				key := *object.Key
				if !strings.HasPrefix(key, prefix) || strings.HasSuffix(key, "/") {
					continue
				}
				relative := strings.TrimPrefix(key, prefix)
				// Reject aliases (a/../b, repeated separators) and foreign-platform paths.
				local, err := filepath.Localize(relative)
				if err != nil || !filepath.IsLocal(local) {
					continue
				}
				missing, err := localMissing(root, local)
				if err != nil {
					return fmt.Errorf("inspect local file %q: %w", key, err)
				}
				if !missing {
					continue
				}
				// A disappeared/replaced configured root must never look like an empty backup.
				original, statErr := root.Stat(".")
				if statErr != nil {
					return statErr
				}
				current, statErr := os.Stat(dir)
				if statErr != nil {
					return statErr
				}
				if !os.SameFile(original, current) {
					return fmt.Errorf("backup root %q changed during sync", dir)
				}
				if aws.ToString(object.ETag) == "" {
					return fmt.Errorf("object %q has no ETag; refusing an unconditional sync deletion", key)
				}
				if s.cfg.DryRun {
					s.l.Info().Str("key", key).Msg("would delete missing file")
					continue
				}
				input := &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.AWS.S3Bucket), Key: object.Key, IfMatch: object.ETag}
				if _, err := s.svc.DeleteObject(s.ctx, input); err != nil {
					return fmt.Errorf("delete %q: %w", key, err)
				}
				s.l.Info().Str("key", key).Msg("deleted missing file")
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *s3clean) pages(prefix string, visit func([]types.Object) error) error {
	input := &s3.ListObjectsV2Input{Bucket: aws.String(s.cfg.AWS.S3Bucket), MaxKeys: aws.Int32(1000)}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	var previous string
	for {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		page, err := s.svc.ListObjectsV2(s.ctx, input)
		if err != nil {
			return fmt.Errorf("list objects: %w", err)
		}
		if page == nil {
			return fmt.Errorf("list objects returned an empty response")
		}
		truncated := aws.ToBool(page.IsTruncated)
		next := aws.ToString(page.NextContinuationToken)
		if truncated && (next == "" || next == previous) {
			return fmt.Errorf("S3 returned an invalid continuation token")
		}
		if len(page.Contents) > 1000 {
			return fmt.Errorf("S3 returned more than 1000 objects in a page")
		}
		if err := visit(page.Contents); err != nil {
			return err
		}
		if !truncated {
			return nil
		}

		previous = next
		input.ContinuationToken = aws.String(next)
	}
}

// A symlink subtree was never backed up, so its remote keys must be retained,
// including dangling symlinks that would otherwise look like missing files.
func localMissing(root *os.Root, local string) (bool, error) {
	parts := strings.Split(local, string(filepath.Separator))
	for i := range parts {
		info, err := root.Lstat(filepath.Join(parts[:i+1]...))
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, nil
		}
		if i < len(parts)-1 && !info.IsDir() {
			return false, fmt.Errorf("parent path is no longer a directory")
		}
	}
	return false, nil
}
