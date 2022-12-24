package s3backup

import (
	"bytes"
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

type S3backuper interface {
	BackupDirectory() (err error)
	SetConfig(cfg models.Config) (err error)
	SetAWSS3(svc s3iface.S3API) (err error)
	SetDirectory(dir string) (err error)
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

//TODO: implement validation
func (b *s3backup) SetConfig(cfg models.Config) (err error) {
	b.cfg = cfg
	if err != nil {
		return err
	}
	return nil
}

//TODO: implement validation
func (b *s3backup) SetAWSS3(svc s3iface.S3API) (err error) {
	b.svc = svc
	if err != nil {
		return err
	}
	return nil
}

//TODO: implement validation
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

	err = filepath.Walk(b.dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			b.l.Fatal().Err(err)
			return err
		}
		s3objectTime, err := b.getS3FileTimestamp(b.cfg, path)
		if err != nil {
			b.l.Fatal().Err(err)
		}
		var s3objectTime2 = *s3objectTime

		localFileTime, err := b.getLocalFileTimestamp(path)

		if localFileTime.Unix() > s3objectTime2.Unix() {
			b.l.Info().Msgf("BACKING UP FILE: %q to AWS S3 storage type %s", path, b.cfg.AWS.StorageClass)
			err = b.uploadFileToS3(b.cfg, path)
		} else {
			b.l.Info().Msgf("SKIPPING File: %q because it has the same or older timestamp than the version in S3", path)
		}

		if err != nil {
			b.l.Fatal().Err(err)
		}
		return nil
	})
	if err != nil {
		b.l.Error().Msgf("error walking the path %q: %v", b.dir, err.Error())
		return
	}
	return
}

func (b *s3backup) getLocalFileTimestamp(file string) (time.Time, error) {
	filestat, err := os.Stat(file)
	if err != nil {
		b.l.Fatal().Err(err)
	}
	return filestat.ModTime(), err
}

func (b *s3backup) getS3FileTimestamp(cfg models.Config, file string) (*time.Time, error) {

	input := &s3.HeadObjectInput{
		Bucket: aws.String(cfg.AWS.S3Bucket),
		Key:    aws.String(file),
	}

	result, err := b.svc.HeadObject(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				epoch := time.Date(1970, 01, 01, 01, 01, 01, 1, time.UTC)
				return &epoch, nil
			default:
				b.l.Fatal().Err(err)
			}
		} else {
			return result.LastModified, err
		}
	}
	return result.LastModified, nil
}

func (b *s3backup) uploadFileToS3(cfg models.Config, fileName string) error {

	fileName, file, err := b.openFile(fileName)
	if err != nil {
		b.l.Fatal().Err(err)
	}

	defer file.Close()

	fileInfo, _ := file.Stat()
	var size = fileInfo.Size()
	buffer := make([]byte, size)
	file.Read(buffer)

	_, s3err := b.svc.PutObject(&s3.PutObjectInput{
		Bucket:               aws.String(cfg.AWS.S3Bucket),
		Key:                  aws.String(fileName),
		ACL:                  aws.String(cfg.AWS.ACL),
		Body:                 bytes.NewReader(buffer),
		ContentLength:        aws.Int64(size),
		ContentType:          aws.String(http.DetectContentType(buffer)),
		ContentDisposition:   aws.String(cfg.AWS.ContentDisposition),
		ServerSideEncryption: aws.String(cfg.AWS.ServerSideEncryption),
		StorageClass:         aws.String(cfg.AWS.StorageClass),
	})

	if s3err != nil {
		b.l.Error().Msgf("PutObject error: %s", s3err.Error())
		return s3err
	}

	return nil
}

func (b *s3backup) openFile(fileName string) (string, *os.File, error) {
	fileName, _ = filepath.Abs(fileName)
	//file := os.FileInfo()
	file, err := os.Open(fileName)
	if err != nil {
		return "", nil, err
	}
	return fileName, file, nil
}
