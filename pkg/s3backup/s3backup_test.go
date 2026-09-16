package s3backup_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/jaysonhurd/s3backup/pkg/s3backup"
	"github.com/jaysonhurd/s3backup/test/fakes/s3api"
)

func TestBackupContentAndKeys(t *testing.T) {
	for _, data := range []string{"", "hello"} {
		t.Run(data, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			file := filepath.Join(dir, "file.txt")
			if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			// A relative backup root must produce exactly the same HEAD and PUT key.
			cwd, _ := os.Getwd()
			relative, err := filepath.Rel(cwd, dir)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(data))
			checksum := base64.StdEncoding.EncodeToString(digest[:])
			var headKey string
			calls := 0
			fake := &s3api.FakeS3API{
				Head: func(_ context.Context, in *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
					headKey = *in.Key
					return nil, &types.NotFound{}
				},
				Put: func(_ context.Context, in *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
					calls++
					if *in.Key != headKey || *in.Key != filepath.ToSlash(file) {
						t.Errorf("inconsistent keys: HEAD %q PUT %q", headKey, *in.Key)
					}
					body, err := io.ReadAll(in.Body)
					if err != nil {
						t.Fatal(err)
					}
					if string(body) != data || aws.ToInt64(in.ContentLength) != int64(len(data)) {
						t.Errorf("incorrect body/length: %q", body)
					}
					if aws.ToString(in.ChecksumSHA256) != checksum || in.Metadata["s3backup-sha256"] != checksum {
						t.Error("missing content checksum")
					}
					if aws.ToString(in.IfNoneMatch) != "*" {
						t.Error("missing conditional create")
					}
					if in.ACL != "" {
						t.Error("default ACL must be omitted")
					}
					return &s3.PutObjectOutput{}, nil
				},
			}
			runner := s3backup.New(models.Config{AWS: models.AWS{S3Bucket: "bucket"}}, fake, relative, nil)
			if err := runner.BackupDirectory(); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("got %d uploads", calls)
			}
		})
	}
}

func TestBackupChangeDetection(t *testing.T) {
	for _, tc := range []struct {
		name, remoteHash string
		wantPut          bool
	}{
		{"same content", hashOf("hello"), false},
		{"changed content with old timestamp", hashOf("other"), true},
		{"legacy object without metadata", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "file")
			if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
				t.Fatal(err)
			}
			old := time.Unix(1, 0)
			if err := os.Chtimes(file, old, old); err != nil {
				t.Fatal(err)
			}
			puts := 0
			fake := &s3api.FakeS3API{
				Head: func(context.Context, *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
					return &s3.HeadObjectOutput{ContentLength: aws.Int64(5), Metadata: map[string]string{"s3backup-sha256": tc.remoteHash}, LastModified: aws.Time(time.Now()), ETag: aws.String("etag")}, nil
				},
				Put: func(_ context.Context, in *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
					puts++
					if aws.ToString(in.IfMatch) != "etag" {
						t.Error("missing conditional update")
					}
					return &s3.PutObjectOutput{}, nil
				},
			}
			if err := s3backup.New(models.Config{AWS: models.AWS{S3Bucket: "bucket"}}, fake, dir, nil).BackupDirectory(); err != nil {
				t.Fatal(err)
			}
			if (puts == 1) != tc.wantPut {
				t.Errorf("uploads = %d", puts)
			}
		})
	}
}
func hashOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return base64.StdEncoding.EncodeToString(h[:])
}

func TestBackupFailures(t *testing.T) {
	for _, tc := range []string{"head", "put", "invalid ACL", "missing root", "cancelled", "changed during upload"} {
		t.Run(tc, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "file")
			if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := models.Config{AWS: models.AWS{S3Bucket: "bucket"}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("request failed")
			puts := 0
			fake := &s3api.FakeS3API{
				Head: func(context.Context, *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
					if tc == "head" {
						return nil, sentinel
					}
					return nil, &types.NotFound{}
				},
				Put: func(context.Context, *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
					puts++
					if tc == "put" {
						return nil, sentinel
					}
					if tc == "changed during upload" {
						if err := os.WriteFile(file, []byte("changed"), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					return &s3.PutObjectOutput{}, nil
				},
			}
			switch tc {
			case "invalid ACL":
				cfg.AWS.ACL = "bogus"
			case "missing root":
				dir = filepath.Join(dir, "missing")
			case "cancelled":
				cancel()
			}
			err := s3backup.NewWithContext(ctx, cfg, fake, dir, nil).BackupDirectory()
			if err == nil {
				t.Fatal("expected failure")
			}
			if (tc == "head" || tc == "put") && !errors.Is(err, sentinel) {
				t.Fatalf("error lost: %v", err)
			}
			if tc != "put" && tc != "changed during upload" && puts != 0 {
				t.Fatal("uploaded after preflight failure")
			}
		})
	}
}

func TestBackupDryRunAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("file", filepath.Join(dir, "link")); err != nil {
		t.Skip(err)
	}
	heads := 0
	fake := &s3api.FakeS3API{Head: func(context.Context, *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
		heads++
		return nil, &types.NotFound{}
	}, Put: func(context.Context, *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
		t.Fatal("dry run uploaded")
		return nil, nil
	}}
	if err := s3backup.New(models.Config{AWS: models.AWS{S3Bucket: "bucket"}, DryRun: true}, fake, dir, nil).BackupDirectory(); err != nil {
		t.Fatal(err)
	}
	if heads != 1 {
		t.Fatalf("examined %d files; symlink must be skipped", heads)
	}
}

func TestOversizedFileFailsBeforeS3(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "large"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(5*1024*1024*1024 + 1); err != nil {
		f.Close()
		t.Skip(err)
	}
	f.Close()
	// No fake methods: any S3 call here is a test failure.
	if err := s3backup.New(models.Config{AWS: models.AWS{S3Bucket: "bucket"}}, &s3api.FakeS3API{}, dir, nil).BackupDirectory(); err == nil {
		t.Fatal("accepted oversized file")
	}
}
