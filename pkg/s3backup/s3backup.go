package s3backup

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
)

const (
	msgWalkFilesystemError    = "error while walking filesystem path"
	msgSkipDirectory          = "skipping directory"
	msgSkipNonRegularFile     = "skipping non-regular file"
	msgGetS3TimestampError    = "error getting S3 file timestamp"
	msgGetLocalTimestampError = "localFileTimestamp failed - continuing"
	msgBackingUpFile          = "backing up file"
	msgUploadToS3Error        = "error uploading file to S3"
	msgSkippingFile           = "skipping file because it has the same or older timestamp than S3"
	msgWalkRootPathError      = "error walking root path"
	msgLocalFileStatError     = "error checking local file timestamp"
	msgOpenFileError          = "error opening file for upload"
	msgReadFileBufferError    = "error reading file bytes for upload"
	msgSeekFileError          = "error seeking file for upload"
	msgInvalidObjectACL       = "invalid object ACL in configuration"
	msgPutObjectError         = "PutObject failed"
)

var objectACLMap = map[string]s3types.ObjectCannedACL{
	"private":                   s3types.ObjectCannedACLPrivate,
	"public-read":               s3types.ObjectCannedACLPublicRead,
	"public-read-write":         s3types.ObjectCannedACLPublicReadWrite,
	"authenticated-read":        s3types.ObjectCannedACLAuthenticatedRead,
	"aws-exec-read":             s3types.ObjectCannedACLAwsExecRead,
	"bucket-owner-read":         s3types.ObjectCannedACLBucketOwnerRead,
	"bucket-owner-full-control": s3types.ObjectCannedACLBucketOwnerFullControl,
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o ../../test/fakes/s3api/s3api.go github.com/jaysonhurd/s3backup/pkg/s3backup/.S3API

var (
	err error
	//file fs.File
	file *os.File
)

type S3backuper interface {
	BackupDirectory() error
	SetConfig(cfg models.Config) error
	SetAWSS3(svc S3API) error
	SetDirectory(dir string) error
}

type S3API interface {
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type s3backup struct {
	cfg models.Config
	svc S3API
	dir string
	l   *zerolog.Logger
}

func New(
	cfg models.Config,
	svc S3API,
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
	return nil
}

func (b *s3backup) SetAWSS3(svc S3API) (err error) {
	b.svc = svc
	return nil
}

func (b *s3backup) SetDirectory(dir string) (err error) {
	b.dir = dir
	return nil
}

// This method backs up an enitre directory structure from the config.json file.
// It will structure the file structure in S3 exactly as it is on the local filesystem.
func (b *s3backup) BackupDirectory() (err error) {

	err = filepath.WalkDir(b.dir, func(path string, info fs.DirEntry, err error) error {

		if err != nil {
			b.l.Error().Err(err).Str("path", path).Msg(msgWalkFilesystemError)
			return err
		}

		if info.IsDir() {
			b.l.Debug().Str("path", path).Msg(msgSkipDirectory)
			return nil
		}

		if !info.Type().IsRegular() {
			b.l.Debug().Str("path", path).Msg(msgSkipNonRegularFile)
			return nil
		}

		s3objectTime, err := b.s3FileTimestamp(b.cfg, path)
		if err != nil {
			b.l.Error().Err(err).Str("path", path).Msg(msgGetS3TimestampError)
			//return err
		}

		// Error checking here is for good measure but would likely never be reached.
		// The WalkDir function would have to find a file, then the file disappear in between
		// (microseconds).  This proved too difficult to write a test for.
		localFileTime, err := b.localFileTimestamp(path)
		if err != nil {
			b.l.Error().Err(err).Str("path", path).Msg(msgGetLocalTimestampError)
		}
		if localFileTime.After(s3objectTime) {
			b.l.Info().Str("path", path).Str("storage_class", b.cfg.AWS.StorageClass).Msg(msgBackingUpFile)
			err = b.uploadFileToS3(path)
			if err != nil {
				b.l.Error().Err(err).Str("path", path).Msg(msgUploadToS3Error)
				//return err
			}
		} else {
			b.l.Info().Str("path", path).Msg(msgSkippingFile)
		}
		return nil
	})

	if err != nil {
		b.l.Error().Err(err).Str("root_dir", b.dir).Msg(msgWalkRootPathError)
		return err
	}
	return nil
}

// localFileTimestamp - gets the file timestamp of a given file on the local filesystem.
func (b *s3backup) localFileTimestamp(file string) (time.Time, error) {
	var filestat fs.FileInfo
	filestat, err = os.Stat(file)
	if err != nil {
		b.l.Error().Err(err).Str("file", file).Msg(msgLocalFileStatError)
		return time.Now(), err
	}
	return filestat.ModTime(), nil
}

// s3FileTimestamp - gets the file timestamp of a given file in S3
func (b *s3backup) s3FileTimestamp(cfg models.Config, file string) (time.Time, error) {

	var (
		result *s3.HeadObjectOutput
		epoch  time.Time
		apiErr smithy.APIError
		nfErr  *s3types.NotFound
	)

	input := s3.HeadObjectInput{
		Bucket: aws.String(cfg.AWS.S3Bucket),
		Key:    aws.String(file),
	}

	result, err = b.svc.HeadObject(context.Background(), &input)

	if err != nil {
		epoch = time.Date(1970, 01, 01, 01, 01, 01, 1, time.UTC)
		if errors.As(err, &nfErr) {
			return epoch, nil
		}
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			return epoch, nil
		}
		return epoch, err
	}

	if result == nil || result.LastModified == nil {
		return time.Date(1970, 01, 01, 01, 01, 01, 1, time.UTC), nil
	}

	return *result.LastModified, nil
}

// s3FileTimestamp - Upload file to S3
func (b *s3backup) uploadFileToS3(fileName string) error {

	fileName, file, err = b.openFile(fileName)
	if err != nil {
		b.l.Error().Err(err).Msg(msgOpenFileError)
		return err
	}

	defer file.Close()

	fileInfo, statErr := file.Stat()
	if statErr != nil {
		b.l.Error().Err(statErr).Msg(msgLocalFileStatError)
		return statErr
	}

	header := make([]byte, 512)
	n, err := file.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		b.l.Error().Err(err).Msg(msgReadFileBufferError)
		return err
	}

	if _, err = file.Seek(0, 0); err != nil {
		b.l.Error().Err(err).Msg(msgSeekFileError)
		return err
	}

	objectACL, err := objectCannedACLFromString(b.cfg.AWS.ACL)
	if err != nil {
		b.l.Error().Err(err).Str("acl", b.cfg.AWS.ACL).Msg(msgInvalidObjectACL)
		return err
	}

	putObject := s3.PutObjectInput{
		Bucket:               aws.String(b.cfg.AWS.S3Bucket),
		Key:                  aws.String(fileName),
		Body:                 file,
		ContentLength:        aws.Int64(fileInfo.Size()),
		ContentType:          aws.String(http.DetectContentType(header[:n])),
		ContentDisposition:   aws.String(b.cfg.AWS.ContentDisposition),
		ServerSideEncryption: s3types.ServerSideEncryption(b.cfg.AWS.ServerSideEncryption),
		StorageClass:         s3types.StorageClass(b.cfg.AWS.StorageClass),
	}
	if objectACL != "" {
		putObject.ACL = objectACL
	}

	_, err = b.svc.PutObject(context.Background(), &putObject)

	if err != nil {
		b.l.Error().Err(err).Msg(msgPutObjectError)
		return err
	}
	return nil
}

func objectCannedACLFromString(acl string) (s3types.ObjectCannedACL, error) {
	trimmed := strings.TrimSpace(acl)
	if trimmed == "" {
		return "", nil
	}
	mapped, ok := objectACLMap[strings.ToLower(trimmed)]
	if !ok {
		return "", errors.New("unsupported object ACL value")
	}
	return mapped, nil
}

func (b *s3backup) openFile(fileName string) (string, *os.File, error) {
	fileName, _ = filepath.Abs(fileName)
	file, err = os.Open(fileName)
	if err != nil {
		return "", nil, err
	}
	return fileName, file, nil
}
