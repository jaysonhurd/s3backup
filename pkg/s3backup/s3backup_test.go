package s3backup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/jaysonhurd/s3backup/pkg/s3backup"
	"github.com/jaysonhurd/s3backup/test/fakes/s3api"
	"github.com/rs/zerolog"
)

var errTestS3 = errors.New("S3 request failed")

// TestBackupDirectory verifies timestamp decisions, upload metadata, and tolerated failures.
func TestBackupDirectory(t *testing.T) {
	tests := []struct {
		name       string
		remote     time.Time
		headErr    error
		putErr     error
		acl        string
		wantUpload bool
	}{
		{name: "new file", wantUpload: true},
		{name: "unchanged file", remote: time.Unix(10000, 0)},
		{name: "newer remote file", remote: time.Unix(20000, 0)},
		{name: "head failure still uploads", headErr: errTestS3, wantUpload: true},
		{name: "missing remote file", headErr: &s3types.NotFound{}, wantUpload: true},
		{name: "upload failure is tolerated", putErr: errTestS3, wantUpload: true},
		{name: "configured ACL", acl: "private", wantUpload: true},
		{name: "normalized ACL", acl: " PRIVATE ", wantUpload: true},
		{name: "invalid ACL", acl: "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			file := filepath.Join(dir, "sample.txt")
			if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
				t.Fatal(err)
			}
			local := time.Unix(10000, 0)
			if err := os.Chtimes(file, local, local); err != nil {
				t.Fatal(err)
			}
			cfg := models.Config{AWS: models.AWS{S3Bucket: "testbucket", ACL: tt.acl, ContentDisposition: "attachment", ServerSideEncryption: "AES256", StorageClass: "STANDARD"}}
			fake := new(s3api.FakeS3API)
			fake.HeadObjectReturns(&s3.HeadObjectOutput{LastModified: &tt.remote}, tt.headErr)
			fake.PutObjectReturns(&s3.PutObjectOutput{}, tt.putErr)
			logger := zerolog.Nop()
			runner := s3backup.New(cfg, fake, dir, &logger)
			if err := runner.BackupDirectory(); err != nil {
				t.Fatalf("BackupDirectory(): %v", err)
			}
			input := fake.LastPutObjectInput
			if (input != nil) != tt.wantUpload {
				t.Fatalf("upload = %v, want %v", input != nil, tt.wantUpload)
			}
			if input == nil {
				return
			}
			if *input.Bucket != cfg.AWS.S3Bucket || *input.Key != file {
				t.Fatalf("unexpected destination: %+v", input)
			}
			if *input.ContentLength != 5 || *input.ContentType != "text/plain; charset=utf-8" {
				t.Fatalf("unexpected content metadata: %+v", input)
			}
			if *input.ContentDisposition != "attachment" || input.ServerSideEncryption != "AES256" || input.StorageClass != "STANDARD" {
				t.Fatalf("unexpected upload options: %+v", input)
			}
			if tt.acl != "" && input.ACL != s3types.ObjectCannedACLPrivate {
				t.Fatalf("ACL = %q", input.ACL)
			}
			if tt.acl == "" && input.ACL != "" {
				t.Fatalf("unexpected ACL %q", input.ACL)
			}
		})
	}
}

// TestBackupDirectoryFilesystem covers empty, missing, and nonregular paths.
func TestBackupDirectoryFilesystem(t *testing.T) {
	dir := t.TempDir()
	fake := new(s3api.FakeS3API)
	runner := s3backup.New(models.Config{}, fake, dir, nil)
	if err := runner.BackupDirectory(); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("missing", link); err == nil {
		if err := runner.BackupDirectory(); err != nil {
			t.Fatal(err)
		}
	}
	if fake.LastPutObjectInput != nil {
		t.Fatal("empty directory or symlink triggered upload")
	}
	if err := runner.SetDirectory(filepath.Join(dir, "missing")); err != nil {
		t.Fatal(err)
	}
	if err := runner.BackupDirectory(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want missing path", err)
	}
}

// TestSetters verifies that changed dependencies and configuration affect the next backup.
func TestSetters(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	runner := s3backup.New(models.Config{}, nil, "missing", nil)
	cfg := models.Config{AWS: models.AWS{S3Bucket: "replacement"}}
	fake := new(s3api.FakeS3API)
	if err := runner.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runner.SetAWSS3(fake); err != nil {
		t.Fatal(err)
	}
	if err := runner.SetDirectory(dir); err != nil {
		t.Fatal(err)
	}
	if err := runner.BackupDirectory(); err != nil {
		t.Fatal(err)
	}
	if fake.LastPutObjectInput == nil || *fake.LastPutObjectInput.Bucket != "replacement" {
		t.Fatal("setters did not update backup dependencies")
	}
}

// TestBackupDirectoryCancellation checks the optional context-aware API.
func TestBackupDirectoryCancellation(t *testing.T) {
	runner := s3backup.New(models.Config{}, new(s3api.FakeS3API), t.TempDir(), nil)
	contextual := runner.(interface{ BackupDirectoryContext(context.Context) error })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := contextual.BackupDirectoryContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want canceled", err)
	}
}
