package s3clean_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/jaysonhurd/s3backup/pkg/s3clean"
	"github.com/jaysonhurd/s3backup/test/fakes/s3api"
)

func TestSyncPaginationAndScope(t *testing.T) {
	dir := t.TempDir()
	prefix := filepath.ToSlash(dir) + "/"
	if err := os.WriteFile(filepath.Join(dir, "present"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	pages, deletes := 0, 0
	fake := &s3api.FakeS3API{
		List: func(_ context.Context, in *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			if aws.ToString(in.Prefix) != prefix {
				t.Fatalf("unsafe prefix: %q", aws.ToString(in.Prefix))
			}
			pages++
			if pages == 1 {
				contents := []types.Object{}
				for i := 0; i < 1000; i++ {
					contents = append(contents, types.Object{Key: aws.String(fmt.Sprintf("%smissing-%d", prefix, i)), ETag: aws.String("etag")})
				}
				return &s3.ListObjectsV2Output{Contents: contents, IsTruncated: aws.Bool(true), NextContinuationToken: aws.String("page2")}, nil
			}
			if pages != 2 || aws.ToString(in.ContinuationToken) != "page2" {
				t.Fatal("incorrect pagination")
			}
			contents := []types.Object{}
			for _, key := range []string{prefix + "last", prefix + "present", "unrelated", prefix + "../escape", prefix + "a//b", prefix + "folder/", prefix[:len(prefix)-1] + "-other/file"} {
				contents = append(contents, types.Object{Key: aws.String(key), ETag: aws.String("etag")})
			}
			return &s3.ListObjectsV2Output{Contents: contents}, nil
		},
		Delete: func(_ context.Context, in *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
			deletes++
			if aws.ToString(in.IfMatch) != "etag" {
				t.Error("missing conditional delete")
			}
			return &s3.DeleteObjectOutput{}, nil
		},
	}
	cfg := models.Config{AWS: models.AWS{S3Bucket: "bucket", BackupDirectories: []string{dir}}}
	if err := s3clean.New(cfg, fake, nil).SyncS3Bucket(); err != nil {
		t.Fatal(err)
	}
	if pages != 2 || deletes != 1001 {
		t.Fatalf("pages=%d deletes=%d", pages, deletes)
	}
}

func TestSyncRefusesMissingRoots(t *testing.T) {
	for _, dirs := range [][]string{nil, {filepath.Join(t.TempDir(), "missing")}} {
		fake := &s3api.FakeS3API{List: func(context.Context, *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			t.Fatal("listed before checking roots")
			return nil, nil
		}}
		if err := s3clean.New(models.Config{AWS: models.AWS{S3Bucket: "bucket", BackupDirectories: dirs}}, fake, nil).SyncS3Bucket(); err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestSyncRefusesSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	if err := os.Symlink(other, filepath.Join(dir, "link")); err != nil {
		t.Skip(err)
	}
	fake := &s3api.FakeS3API{
		List: func(context.Context, *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{Contents: []types.Object{{Key: aws.String(filepath.ToSlash(dir) + "/link/missing")}}}, nil
		},
		Delete: func(context.Context, *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
			t.Fatal("deleted through escaping symlink")
			return nil, nil
		},
	}
	if err := s3clean.New(models.Config{AWS: models.AWS{S3Bucket: "bucket", BackupDirectories: []string{dir}}}, fake, nil).SyncS3Bucket(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupErrorsAndDryRun(t *testing.T) {
	for _, operation := range []string{"sync", "wipe"} {
		for _, scenario := range []string{"list error", "delete error", "partial error", "dry run", "nil key", "invalid pagination"} {
			t.Run(operation+"/"+scenario, func(t *testing.T) {
				dir := t.TempDir()
				sentinel := errors.New("AWS failure")
				deletes := 0
				fake := &s3api.FakeS3API{
					List: func(context.Context, *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
						if scenario == "list error" {
							return nil, sentinel
						}
						key := aws.String(filepath.ToSlash(dir) + "/missing")
						if scenario == "nil key" {
							key = nil
						}
						out := &s3.ListObjectsV2Output{Contents: []types.Object{{Key: key, ETag: aws.String("etag")}}}
						if scenario == "invalid pagination" {
							out.IsTruncated = aws.Bool(true)
						}
						return out, nil
					},
					Delete: func(context.Context, *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
						deletes++
						if scenario == "delete error" {
							return nil, sentinel
						}
						return &s3.DeleteObjectOutput{}, nil
					},
					DeleteBatch: func(context.Context, *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error) {
						deletes++
						if scenario == "delete error" {
							return nil, sentinel
						}
						if scenario == "partial error" {
							return &s3.DeleteObjectsOutput{Errors: []types.Error{{Key: aws.String("key"), Code: aws.String("AccessDenied"), Message: aws.String("denied")}}}, nil
						}
						return &s3.DeleteObjectsOutput{}, nil
					},
				}
				cfg := models.Config{AWS: models.AWS{S3Bucket: "bucket", BackupDirectories: []string{dir}}, DryRun: scenario == "dry run"}
				cleaner := s3clean.New(cfg, fake, nil)
				var err error
				if operation == "sync" {
					err = cleaner.SyncS3Bucket()
				} else {
					err = cleaner.WipeS3Bucket()
				}
				wantErr := scenario != "dry run" && !(scenario == "partial error" && operation == "sync")
				if (err != nil) != wantErr {
					t.Fatalf("err=%v wantErr=%v", err, wantErr)
				}
				if scenario == "dry run" && deletes != 0 {
					t.Fatal("dry run deleted objects")
				}
				if (scenario == "list error" || scenario == "delete error") && !errors.Is(err, sentinel) {
					t.Fatalf("error lost: %v", err)
				}
			})
		}
	}
}

func TestWipePagination(t *testing.T) {
	pages, deletes := 0, 0
	fake := &s3api.FakeS3API{
		List: func(_ context.Context, in *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			pages++
			if in.Prefix != nil {
				t.Fatal("wipe must cover all current keys")
			}
			return &s3.ListObjectsV2Output{Contents: []types.Object{{Key: aws.String(fmt.Sprint(pages))}}, IsTruncated: aws.Bool(pages == 1), NextContinuationToken: aws.String("next")}, nil
		},
		DeleteBatch: func(_ context.Context, in *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error) {
			deletes += len(in.Delete.Objects)
			return &s3.DeleteObjectsOutput{}, nil
		},
	}
	if err := s3clean.New(models.Config{AWS: models.AWS{S3Bucket: "bucket"}}, fake, nil).WipeS3Bucket(); err != nil {
		t.Fatal(err)
	}
	if deletes != 2 || pages != 2 {
		t.Fatalf("pages=%d deletes=%d", pages, deletes)
	}
}

func TestCancelledCleanupDoesNotCallS3(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := models.Config{AWS: models.AWS{S3Bucket: "bucket", BackupDirectories: []string{t.TempDir()}}}
	cleaner := s3clean.NewWithContext(ctx, cfg, &s3api.FakeS3API{}, nil)
	if err := cleaner.WipeS3Bucket(); !errors.Is(err, context.Canceled) {
		t.Fatalf("wipe: %v", err)
	}
	if err := cleaner.SyncS3Bucket(); !errors.Is(err, context.Canceled) {
		t.Fatalf("sync: %v", err)
	}
}
