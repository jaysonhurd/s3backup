package s3backup_test

import (
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

var (
	l             = zerolog.Nop()
	fakes3api     *s3api.FakeS3API
	backup        s3backup.S3backuper
	svc           *s3.Client
	testDirectory = "test"
	cfg           models.Config
	err           error
	myTime        time.Time
)

func TestBackupDirectoryNothingToDo(t *testing.T) {

	cfg = models.Config{
		AWS: models.AWS{
			S3Region:             "somewhere",
			S3Bucket:             "testbucket",
			SecretAccessKey:      "xxxxxxx",
			AccessKeyId:          "yyyyyyy",
			BackupDirectories:    nil,
			ACL:                  "",
			ContentDisposition:   "",
			ServerSideEncryption: "",
			StorageClass:         "",
		},
		Logging: models.Logging{},
	}
	fakes3api = new(s3api.FakeS3API)
	fakes3api.PutObjectReturns(&s3.PutObjectOutput{}, nil)
	lastModified, err := time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
	fakes3api.HeadObjectReturns(&s3.HeadObjectOutput{LastModified: &lastModified}, nil)
	myTime, _ = time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
	myS3 := s3backup.New(cfg, fakes3api, testDirectory, &l)

	err = myS3.BackupDirectory()
	if err != nil {
		t.Fail()
	}
}

func TestBackupDirectoryNonExistentDirectory(t *testing.T) {

	cfg = models.Config{
		AWS: models.AWS{
			S3Region:             "somewhere",
			S3Bucket:             "testbucket",
			SecretAccessKey:      "xxxxxxx",
			AccessKeyId:          "yyyyyyy",
			BackupDirectories:    nil,
			ACL:                  "",
			ContentDisposition:   "",
			ServerSideEncryption: "",
			StorageClass:         "",
		},
		Logging: models.Logging{},
	}
	fakes3api = new(s3api.FakeS3API)
	fakes3api.PutObjectReturns(&s3.PutObjectOutput{}, nil)
	lastModified, err := time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
	fakes3api.HeadObjectReturns(&s3.HeadObjectOutput{LastModified: &lastModified}, nil)
	myTime, _ = time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
	myS3 := s3backup.New(cfg, fakes3api, "nodirectory", &l)

	err = myS3.BackupDirectory()
	if err == nil {
		t.Fail()
	}
}

func TestBackupDirectoryHeadObjectFails(t *testing.T) {

	cfg = models.Config{
		AWS: models.AWS{
			S3Region:             "somewhere",
			S3Bucket:             "testbucket",
			SecretAccessKey:      "xxxxxxx",
			AccessKeyId:          "yyyyyyy",
			BackupDirectories:    nil,
			ACL:                  "",
			ContentDisposition:   "",
			ServerSideEncryption: "",
			StorageClass:         "",
		},
		Logging: models.Logging{},
	}
	fakes3api = new(s3api.FakeS3API)
	//fakes3api.PutObjectReturns(&s3.PutObjectOutput{}, nil)
	//lastModified, err := time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
	fakes3api.HeadObjectReturns(&s3.HeadObjectOutput{}, errors.New("something went wrong"))
	myTime, _ = time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
	myS3 := s3backup.New(cfg, fakes3api, testDirectory, &l)

	err = myS3.BackupDirectory()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestSetConfig(t *testing.T) {

	setConfigTests := []struct {
		name        string
		dir         string
		expectedErr error
		cfg         models.Config
	}{
		{name: "test1", expectedErr: nil, cfg: models.Config{
			AWS:     models.AWS{},
			Logging: models.Logging{},
		},
			dir: "cases",
		},
	}
	for _, cases := range setConfigTests {
		backup = s3backup.New(cases.cfg, svc, cases.dir, &l)
		err = backup.SetConfig(cases.cfg)
		if err != cases.expectedErr {
			t.Errorf("wanted %q: got: %v", cases.expectedErr, err.Error())
		}
	}
}

func TestSetAWSS3(t *testing.T) {
	tests := []struct {
		name   string
		expect error
		cfg    models.Config
	}{
		{name: "test1", expect: nil, cfg: models.Config{
			AWS:     models.AWS{},
			Logging: models.Logging{},
		},
		},
	}
	for _, test := range tests {
		backup = s3backup.New(test.cfg, svc, "/etc", &l)
		err = backup.SetAWSS3(svc)
		if err != nil {
			t.Errorf("wanted %q: got: %v", test.expect, err.Error())
		}
	}
}

func TestSetDirectory(t *testing.T) {
	tests := []struct {
		name   string
		expect error
		cfg    models.Config
	}{
		{name: "test1", expect: nil, cfg: models.Config{
			AWS:     models.AWS{},
			Logging: models.Logging{},
		},
		},
	}
	for _, test := range tests {
		backup = s3backup.New(test.cfg, svc, "/etc", &l)
		err = backup.SetDirectory("")
		if err != nil {
			t.Errorf("wanted %q: got: %v", test.expect, err.Error())
		}
	}
}

func TestBackupDirectorySetsConfiguredACL(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "sample.txt")
	if writeErr := os.WriteFile(tmpFile, []byte("hello"), 0o600); writeErr != nil {
		t.Fatalf("unable to create temp file: %v", writeErr)
	}

	cfg = models.Config{
		AWS: models.AWS{
			S3Region:             "us-east-1",
			S3Bucket:             "testbucket",
			ACL:                  "private",
			ContentDisposition:   "attachment",
			ServerSideEncryption: "AES256",
			StorageClass:         "STANDARD",
		},
		Logging: models.Logging{},
	}

	fakes3api = new(s3api.FakeS3API)
	fakes3api.PutObjectReturns(&s3.PutObjectOutput{}, nil)
	old := time.Unix(0, 0).UTC()
	fakes3api.HeadObjectReturns(&s3.HeadObjectOutput{LastModified: &old}, nil)

	backupRunner := s3backup.New(cfg, fakes3api, tmpDir, &l)
	if backupErr := backupRunner.BackupDirectory(); backupErr != nil {
		t.Fatalf("BackupDirectory() returned unexpected error: %v", backupErr)
	}

	if fakes3api.LastPutObjectInput == nil {
		t.Fatalf("expected PutObject to be called")
	}
	if fakes3api.LastPutObjectInput.ACL != s3types.ObjectCannedACLPrivate {
		t.Fatalf("expected ACL %q, got %q", s3types.ObjectCannedACLPrivate, fakes3api.LastPutObjectInput.ACL)
	}
}

func TestBackupDirectorySkipsPutObjectOnInvalidACL(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "sample.txt")
	if writeErr := os.WriteFile(tmpFile, []byte("hello"), 0o600); writeErr != nil {
		t.Fatalf("unable to create temp file: %v", writeErr)
	}

	cfg = models.Config{
		AWS: models.AWS{
			S3Region:             "us-east-1",
			S3Bucket:             "testbucket",
			ACL:                  "not-a-real-acl",
			ContentDisposition:   "attachment",
			ServerSideEncryption: "AES256",
			StorageClass:         "STANDARD",
		},
		Logging: models.Logging{},
	}

	fakes3api = new(s3api.FakeS3API)
	fakes3api.PutObjectReturns(&s3.PutObjectOutput{}, nil)
	old := time.Unix(0, 0).UTC()
	fakes3api.HeadObjectReturns(&s3.HeadObjectOutput{LastModified: &old}, nil)

	backupRunner := s3backup.New(cfg, fakes3api, tmpDir, &l)
	if backupErr := backupRunner.BackupDirectory(); backupErr != nil {
		t.Fatalf("BackupDirectory() returned unexpected error: %v", backupErr)
	}

	if fakes3api.LastPutObjectInput != nil {
		t.Fatalf("expected PutObject not to be called for invalid ACL")
	}
}
