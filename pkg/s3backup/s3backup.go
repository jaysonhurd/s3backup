package s3backup

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
)

const checksumMetadata = "s3backup-sha256"
const maxPutSize int64 = 5 * 1024 * 1024 * 1024

type S3backuper interface {
	BackupDirectory() error
	SetConfig(models.Config) error
	SetAWSS3(S3API) error
	SetDirectory(string) error
}

type S3API interface {
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type s3backup struct {
	ctx context.Context
	cfg models.Config
	svc S3API
	dir string
	l   *zerolog.Logger
}

func New(cfg models.Config, svc S3API, dir string, l *zerolog.Logger) S3backuper {
	return NewWithContext(context.Background(), cfg, svc, dir, l)
}

func NewWithContext(ctx context.Context, cfg models.Config, svc S3API, dir string, l *zerolog.Logger) S3backuper {
	if l == nil {
		logger := zerolog.Nop()
		l = &logger
	}
	return &s3backup{ctx: ctx, cfg: cfg, svc: svc, dir: dir, l: l}
}
func (b *s3backup) SetConfig(cfg models.Config) error { b.cfg = cfg; return nil }
func (b *s3backup) SetAWSS3(svc S3API) error          { b.svc = svc; return nil }
func (b *s3backup) SetDirectory(dir string) error     { b.dir = dir; return nil }

// BackupDirectory preserves the existing absolute-path key layout (including
// the leading slash on Unix). Symbolic links and special files are not uploaded.
func (b *s3backup) BackupDirectory() error {
	if err := b.cfg.Validate(); err != nil {
		return err
	}
	roots, err := models.BackupRoots([]string{b.dir})
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(roots[0])
	if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if err := b.ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		key := filepath.ToSlash(filepath.Join(roots[0], filepath.FromSlash(path)))
		if err := b.backupFile(root, path, key); err != nil {
			return fmt.Errorf("backup %q: %w", key, err)
		}
		return nil
	})
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (b *s3backup) backupFile(root *os.Root, path, key string) error {
	f, err := root.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("file is no longer regular")
	}
	if info.Size() > maxPutSize {
		return fmt.Errorf("file exceeds single-PUT limit of 5 GiB; multipart upload is required")
	}
	hash := sha256.New()
	size, err := io.Copy(hash, contextReader{b.ctx, f})
	if err != nil {
		return err
	}
	after, err := f.Stat()
	if err != nil {
		return err
	}
	if size != info.Size() || !unchanged(info, after) {
		return fmt.Errorf("file changed while hashing; retry with a stable source")
	}
	checksum := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	head, err := b.svc.HeadObject(b.ctx, &s3.HeadObjectInput{Bucket: aws.String(b.cfg.AWS.S3Bucket), Key: aws.String(key)})
	missing := isNotFound(err)
	if err != nil && !missing {
		return fmt.Errorf("head object: %w", err)
	}
	if !missing && head == nil {
		return fmt.Errorf("head object returned an empty response")
	}
	after, err = f.Stat()
	if err != nil {
		return err
	}
	if !unchanged(info, after) {
		return fmt.Errorf("file changed while checking S3; retry with a stable source")
	}
	if !missing && head.Metadata[checksumMetadata] == checksum && head.ContentLength != nil && *head.ContentLength == size {
		b.l.Debug().Str("key", key).Msg("unchanged file")
		return nil
	}
	if !missing && aws.ToString(head.ETag) == "" {
		return fmt.Errorf("head object did not return an ETag; refusing an unconditional overwrite")
	}
	if b.cfg.DryRun {
		b.l.Info().Str("key", key).Msg("would upload file")
		return nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	header := make([]byte, 512)
	n, err := io.ReadFull(f, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	input := &s3.PutObjectInput{
		Bucket: aws.String(b.cfg.AWS.S3Bucket), Key: aws.String(key), Body: f,
		ContentLength: aws.Int64(size), ContentType: aws.String(http.DetectContentType(header[:n])),
		ACL: types.ObjectCannedACL(b.cfg.AWS.ACL), StorageClass: types.StorageClass(b.cfg.AWS.StorageClass),
		ServerSideEncryption: types.ServerSideEncryption(b.cfg.AWS.ServerSideEncryption),
		ChecksumSHA256:       aws.String(checksum), ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		Metadata: map[string]string{checksumMetadata: checksum, "s3backup-mtime": info.ModTime().UTC().Format(time.RFC3339Nano)},
	}
	if b.cfg.AWS.ContentDisposition != "" {
		input.ContentDisposition = aws.String(b.cfg.AWS.ContentDisposition)
	}
	if b.cfg.AWS.SSEKMSKeyID != "" {
		input.SSEKMSKeyId = aws.String(b.cfg.AWS.SSEKMSKeyID)
	}
	// Fail if another writer changes the object between inspection and upload.
	if missing {
		input.IfNoneMatch = aws.String("*")
	} else if head.ETag != nil {
		input.IfMatch = head.ETag
	}
	if _, err := b.svc.PutObject(b.ctx, input); err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	after, err = f.Stat()
	if err != nil {
		return err
	}
	if !unchanged(info, after) {
		return fmt.Errorf("file changed during upload; retry with a stable source")
	}
	b.l.Info().Str("key", key).Int64("bytes", size).Msg("uploaded file")
	return nil
}

func unchanged(a, b fs.FileInfo) bool { return a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) }
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var notFound *types.NotFound
	var api smithy.APIError
	var response *smithyhttp.ResponseError
	return errors.As(err, &notFound) ||
		(errors.As(err, &api) && (api.ErrorCode() == "NotFound" || api.ErrorCode() == "NoSuchKey")) ||
		(errors.As(err, &response) && response.HTTPStatusCode() == http.StatusNotFound)
}
