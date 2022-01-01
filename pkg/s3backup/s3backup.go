package s3backup

import (
	"bytes"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
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

type S3backup interface {
	BackupDirectory()
	SetConfig(cfg models.Config)
	SetAWSSession(sess session.Session)
	SetDirectory(dir string)
}

type s3backup struct {
	cfg     models.Config
	awsSess session.Session
	Client  s3iface.S3API
	dir     string
	l       *zerolog.Logger
}

// Constructor for backing up a single directory
func CreateBackup(cfg models.Config, sess session.Session, dir string, logger *zerolog.Logger) *s3backup {
	return &s3backup{
		cfg:     cfg,
		awsSess: sess,
		dir:     dir,
		l:       logger,
	}
}

//TODO: implement validation
func (b *s3backup) SetConfig(cfg models.Config) {
	b.cfg = cfg
}

//TODO: implement validation
func (b *s3backup) SetAWSSession(sess session.Session) {
	b.awsSess = sess
}

//TODO: implement validation
func (b *s3backup) SetDirectory(dir string) {
	b.dir = dir
}

// This method backs up an enitre directory structure from the config.json file.
// It will structure the file structure in S3 exactly as it is on the local filesystem.
func (b *s3backup) BackupDirectory() {

	err := filepath.Walk(b.dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			b.l.Fatal().Err(err)
			return err
		}
		s3objectTime, err := getS3FileTimestamp(&b.cfg, &b.awsSess, path, b.l)
		if err != nil {
			b.l.Fatal().Err(err)
		}
		var s3objectTime2 = *s3objectTime

		localFileTime, err := getLocalFileTimestamp(path, b.l)

		if localFileTime.Unix() > s3objectTime2.Unix() {
			b.l.Info().Msgf("BACKING UP FILE: %q to AWS S3 storage type %s", path, b.cfg.AWS.StorageClass)
			err = uploadFileToS3(&b.cfg, &b.awsSess, path, b.l)
		} else {
			b.l.Info().Msgf("SKIPPING File: %q because it has the same or older timestamp than the version in S3", path)
		}

		if err != nil {
			b.l.Fatal().Err(err)
		}
		return nil
	})
	if err != nil {
		b.l.Warn().Msgf("error walking the path %q: %v", err.Error())
		return
	}
	return
}

func getLocalFileTimestamp(file string, l *zerolog.Logger) (time.Time, error) {
	filestat, err := os.Stat(file)
	if err != nil {
		l.Fatal().Err(err)
	}
	return filestat.ModTime(), err
}

func getS3FileTimestamp(cfg *models.Config, sess *session.Session, file string, l *zerolog.Logger) (*time.Time, error) {

	svc := s3.New(sess)

	input := &s3.HeadObjectInput{
		Bucket: aws.String(cfg.AWS.S3Bucket),
		Key:    aws.String(file),
	}

	result, err := svc.HeadObject(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				epoch := time.Date(1970, 01, 01, 01, 01, 01, 1, time.UTC)
				return &epoch, nil
			default:
				l.Fatal().Err(err)
			}
		} else {
			return result.LastModified, err
		}
	}
	return result.LastModified, nil
}

func uploadFileToS3(cfg *models.Config, sess *session.Session, fileName string, l *zerolog.Logger) error {

	fileName, file, err := openFile(fileName)
	if err != nil {
		l.Fatal().Err(err)
	}

	defer file.Close()

	fileInfo, _ := file.Stat()
	var size = fileInfo.Size()
	buffer := make([]byte, size)
	file.Read(buffer)

	_, s3err := s3.New(sess).PutObject(&s3.PutObjectInput{
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

	return s3err
}

func openFile(fileName string) (string, *os.File, error) {
	fileName, _ = filepath.Abs(fileName)
	file, err := os.Open(fileName)
	if err != nil {
		return "", nil, err
	}
	return fileName, file, nil
}
