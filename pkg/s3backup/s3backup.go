package s3backup

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o ../../test/fakes/s3api/s3api.go github.com/aws/aws-sdk-go/service/s3/s3iface/.S3API

var (
	err error
	//file fs.File
	file *os.File
)

type S3backuper interface {
	BackupDirectory() error
	SetConfig(cfg models.Config) error
	SetAWSS3(svc s3iface.S3API) error
	SetDirectory(dir string) error
}

type s3backup struct {
	cfg models.Config
	svc s3iface.S3API
	dir string
	l   *zerolog.Logger
}

func New(
	cfg models.Config,
	svc s3iface.S3API,
	dir string,
	l *zerolog.Logger,
) S3backuper {
	return &s3backup{
		cfg: cfg,
		svc: svc,
		dir: dir,
		l:   l,
	}
}

func (b *s3backup) SetConfig(cfg models.Config) (err error) {
	b.cfg = cfg
	if err != nil {
		return err
	}
	return nil
}

func (b *s3backup) SetAWSS3(svc s3iface.S3API) (err error) {
	b.svc = svc
	if err != nil {
		return err
	}
	return nil
}

func (b *s3backup) SetDirectory(dir string) (err error) {
	b.dir = dir
	if err != nil {
		return err
	}
	return nil
}

// This method backs up an enitre directory structure from the config.json file.
// It will structure the file structure in S3 exactly as it is on the local filesystem.
func (b *s3backup) BackupDirectory() (err error) {

	err = filepath.WalkDir(b.dir, func(path string, info fs.DirEntry, err error) error {

		if err != nil {
			b.l.Error().Msgf("error with walking filesystem %v: %v ", path, err.Error())
			return err
		}

		s3objectTime, err := b.s3FileTimestamp(b.cfg, path)
		if err != nil {
			b.l.Error().Msgf("error with getting s3FileTimeStamp: %v ", err.Error())
			return err
		}

		// Error checking here is for good measure but would likely never be reached.
		// The WalkDir function would have to find a file, then the file disappear in between
		// (microseconds).  This proved too difficult to write a test for.
		localFileTime, err := b.localFileTimestamp(path)
		if err != nil {
			b.l.Error().Msgf("localFileTimestamp error on %v - error is: %v - CONTINUING", path, err.Error())
		}
		if localFileTime.Unix() > s3objectTime.Unix() {
			b.l.Info().Msgf("BACKING UP FILE: %q to AWS S3 storage type %s", path, b.cfg.AWS.StorageClass)
			err = b.uploadFileToS3(path)
			if err != nil {
				b.l.Error().Msgf("error uploading file to S3: %v", err.Error())
				//return err
			}
		} else {
			b.l.Info().Msgf("SKIPPING File: %q because it has the same or older timestamp than the version in S3", path)
		}
		fmt.Println(path)
		return nil
	})

	if err != nil {
		b.l.Error().Msgf("error walking the path %q: %v", b.dir, err.Error())
		return err
	}
	return nil
}

func (b *s3backup) localFileTimestamp(file string) (time.Time, error) {
	var filestat fs.FileInfo
	filestat, err = os.Stat(file)
	if err != nil {
		b.l.Error().Msgf("error checking local file timestamp on file %v, erorr is %v", file, err.Error())
		return time.Now(), err
	}
	return filestat.ModTime(), nil
}

func (b *s3backup) s3FileTimestamp(cfg models.Config, file string) (time.Time, error) {

	var (
		result *s3.HeadObjectOutput
		epoch  time.Time
		aerr   awserr.Error
		ok     bool
	)

	input := &s3.HeadObjectInput{
		Bucket: aws.String(cfg.AWS.S3Bucket),
		Key:    aws.String(file),
	}

	result, err = b.svc.HeadObject(input)

	if err != nil {
		if aerr, ok = err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				epoch = time.Date(1970, 01, 01, 01, 01, 01, 1, time.UTC)
				return epoch, nil
			default:
				epoch = time.Date(1970, 01, 01, 01, 01, 01, 1, time.UTC)
				return epoch, err
			}
		} else {
			return *result.LastModified, err
		}
	}

	return *result.LastModified, nil
}

func (b *s3backup) uploadFileToS3(fileName string) error {

	fileName, file, err = b.openFile(fileName)
	if err != nil {
		b.l.Error().Err(err)
		return err
	}

	defer file.Close()

	fileInfo, _ := file.Stat()
	//var size = fileInfo.Size()
	buffer := make([]byte, fileInfo.Size())
	file.Read(buffer)

	_, err = b.svc.PutObject(&s3.PutObjectInput{
		Bucket:               aws.String(b.cfg.AWS.S3Bucket),
		Key:                  aws.String(fileName),
		ACL:                  aws.String(b.cfg.AWS.ACL),
		Body:                 bytes.NewReader(buffer),
		ContentLength:        aws.Int64(fileInfo.Size()),
		ContentType:          aws.String(http.DetectContentType(buffer)),
		ContentDisposition:   aws.String(b.cfg.AWS.ContentDisposition),
		ServerSideEncryption: aws.String(b.cfg.AWS.ServerSideEncryption),
		StorageClass:         aws.String(b.cfg.AWS.StorageClass),
	})

	if err != nil {
		b.l.Error().Msgf("PutObject error: %s", err.Error())
		return err
	}
	return nil
}

func (b *s3backup) openFile(fileName string) (string, *os.File, error) {
	fileName, _ = filepath.Abs(fileName)
	file, err = os.Open(fileName)
	if err != nil {
		return "", nil, err
	}
	return fileName, file, nil
}
