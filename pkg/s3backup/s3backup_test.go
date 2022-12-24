package s3backup_test

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/jaysonhurd/s3backup/pkg/s3backup"
	"github.com/jaysonhurd/s3backup/pkg/utilities"
	"github.com/rs/zerolog"
	"log"
	"testing"
	"testing/fstest"
	"time"
)

var (
	l       = zerolog.Nop()
	awsSess *session.Session
	s3api   s3iface.S3API
	svc     *s3.S3
	cfg     models.Config
	dirTest fstest.MapFS
	err     error
	myTime  time.Time
)

type MockS3Client interface {
	HeadObject(_ *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)
}

type mockS3Client struct {
	s3iface.S3API
}

func (m *mockS3Client) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	log.Println("Mock Uploaded to S3:", *input.Key)
	return &s3.PutObjectOutput{}, nil

}

func (m *mockS3Client) HeadObject(_ *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	var result s3.HeadObjectOutput

	myTime, _ := time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
	result.LastModified = &myTime
	if err != nil {
		return &result, err
	}

	return &result, nil
}

func TestBackupDirectory(t *testing.T) {
	myTime, _ = time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
	mapFile1 := &fstest.MapFile{
		Data:    []byte(`test/test1.go"`),
		Mode:    0,
		ModTime: myTime,
		Sys:     nil,
	}
	mapFile2 := &fstest.MapFile{
		Data:    []byte(`test/test2.go"`),
		Mode:    0,
		ModTime: time.Time{},
		Sys:     nil,
	}
	dirTest = make(map[string]*fstest.MapFile)
	dirTest["test1"] = mapFile1
	dirTest["test2"] = mapFile2

	sess, err := utilities.CreateAWSSession(cfg, &l)
	apiclient := s3.New(sess)
	mockSvc := &mockS3Client{apiclient}
	backup := s3backup.New(cfg, mockSvc, "test", &l)
	err = backup.BackupDirectory()
	if err != nil {
		t.Fail()
	}
}

func TestSetConfig(t *testing.T) {
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
		//awsSess, _ = utilities.CreateAWSSession(cfg, &l)
		//svc = s3.New(awsSess)
		backup := s3backup.New(test.cfg, svc, "/etc", &l)
		err = backup.SetConfig(test.cfg)
		if err != nil {
			t.Errorf("wanted %q: got: %v", test.expect, err.Error())
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
		backup := s3backup.New(test.cfg, svc, "/etc", &l)
		err = backup.SetAWSS3(s3api)
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
		backup := s3backup.New(test.cfg, svc, "/etc", &l)
		err = backup.SetDirectory("")
		if err != nil {
			t.Errorf("wanted %q: got: %v", test.expect, err.Error())
		}
	}
}
