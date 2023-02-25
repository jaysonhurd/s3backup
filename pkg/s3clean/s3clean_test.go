package s3clean_test

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/jaysonhurd/s3backup/pkg/s3clean"
	"github.com/jaysonhurd/s3backup/pkg/utilities"
	"github.com/jaysonhurd/s3backup/test/fakes/s3api"
	"github.com/rs/zerolog"
	"github.com/spf13/afero"
	"log"
	"testing"
	"testing/fstest"
	"time"
)

var (
	l             = zerolog.Nop()
	sess          *session.Session
	fakes3api     *s3api.FakeS3API
	clean         s3clean.S3Cleaner
	svc           *s3.S3
	testDirectory = "test"
	mockSvc       *mockS3Client
	cfg           models.Config
	dirTest       fstest.MapFS
	testDirs      map[string]*fstest.MapFile
	err           error
	myTime        time.Time
	FS            = afero.NewMemMapFs()
	AFS           = &afero.Afero{Fs: FS}
)

func TestWipeBucket(t *testing.T) {

	fakes3api = new(s3api.FakeS3API)
	myTime, _ = time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
	tests := []struct {
		name        string
		mytime      time.Time
		mockSvc     *mockS3Client
		dir         string
		files       map[string]*fstest.MapFile
		expectedErr error
	}{
		{
			name:    "SUCCEED_SKIP_S3_BECAUSE_FILE_IS_NEWER",
			mockSvc: &mockS3Client{putObject: true, headObject: true, s3IsNewer: true, s3FileExists: true},
			mytime:  myTime,
			dir:     testDirectory,
			//dir:         "./gotest",
			expectedErr: nil,
		},
		//{
		//	name:        "SUCCEED_FILE_DOESNT_EXIST_ON_AWS",
		//	mockSvc:     &mockS3Client{putObject: true, headObject: false, s3IsNewer: true, s3FileExists: false},
		//	mytime:      myTime,
		//	dir:         testDirectory,
		//	expectedErr: nil,
		//},
		//{
		//	name:        "SUCCEED_FILE_IS_OLDER",
		//	mockSvc:     &mockS3Client{putObject: true, headObject: true, s3IsNewer: false, s3FileExists: true},
		//	mytime:      time.Now(),
		//	dir:         testDirectory,
		//	expectedErr: nil,
		//},
		//{
		//	name:        "FAIL_S3_PUTOBJECT_FAIL",
		//	mockSvc:     &mockS3Client{putObject: false, headObject: true, s3IsNewer: false, s3FileExists: true},
		//	mytime:      myTime,
		//	dir:         testDirectory,
		//	expectedErr: errors.New("putobject error"),
		//},
		//{
		//	name:        "FAIL_S3_HEADOBJECT_FAIL",
		//	mockSvc:     &mockS3Client{putObject: true, headObject: false, s3IsNewer: true, s3FileExists: true},
		//	mytime:      myTime,
		//	dir:         testDirectory,
		//	expectedErr: errors.New("headobject error"),
		//},
		//{
		//	name:        "FAIL_NONEXISTENT_DIRECTORY",
		//	mockSvc:     &mockS3Client{putObject: false, headObject: true, s3IsNewer: true, s3FileExists: true},
		//	mytime:      myTime,
		//	dir:         "nonexistent",
		//	expectedErr: errors.New("lstat nonexistent: no such file or directory"),
		//},
	}

	for _, cases := range tests {
		t.Run(cases.name, func(t *testing.T) {
			sess, err = utilities.CreateAWSSession(cfg, &l)
			svc = s3.New(sess)
			clean = s3clean.New(cfg, cases.mockSvc, &l)
			err = clean.WipeS3Bucket()
			if err != nil {
				if err.Error() != cases.expectedErr.Error() {
					t.Errorf("expected %v, got %v", cases.expectedErr.Error(), err.Error())
					t.Fail()
				}
			}

		})
	}
}

type MockS3Client interface {
	HeadObject(_ *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)
	PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
}

type mockS3Client struct {
	s3api.FakeS3API
	putObject    bool
	headObject   bool
	s3IsNewer    bool
	s3FileExists bool
}

func (m *mockS3Client) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	log.Println("Mock Uploaded to PHONY S3:", *input.Key)
	if m.putObject {
		return &s3.PutObjectOutput{}, nil
	} else {
		return &s3.PutObjectOutput{}, errors.New("putobject error")
	}

	return &s3.PutObjectOutput{}, nil

}

func (m *mockS3Client) HeadObject(_ *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	var result s3.HeadObjectOutput
	//var err error

	if !m.headObject {
		if !m.s3FileExists {
			return &result, awserr.New("NotFound", "", nil)
		}
		myTime, _ = time.Parse("2 Jan 06 03:04PM", "10 Nov 10 11:00PM")
		result.LastModified = &myTime
		return &result, errors.New("headobject error")
	}

	if m.s3IsNewer {
		myTime = time.Now()
		result.LastModified = &myTime
		return &result, nil
	} else {
		myTime, _ = time.Parse("2 Jan 2006 03:04PM", "10 Nov 1970 11:00PM")
		result.LastModified = &myTime
		return &result, nil
	}

}
