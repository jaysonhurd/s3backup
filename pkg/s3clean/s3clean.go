package s3clean

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
)

const (
	msgListObjectsBucketError = "Unable to list objects in bucket"
	msgDeleteObjectsBucketErr = "Unable to delete objects from bucket"
	msgMissingLocalFile       = "local file does not exist, attempting S3 removal"
	msgUnableToRemoveS3File   = "unable to remove object from S3; continuing"
	msgRemovedFromS3          = "removed object from S3"
	msgDeleteObjectFailed     = "delete object failed"
	msgNoSuchBucket           = "NoSuchBucket"
	msgListObjectsFailed      = "list objects failed"
)

type S3Cleaner interface {
	SyncS3Bucket() (err error)
	WipeS3Bucket() (err error)
}

type s3clean struct {
	cfg models.Config
	svc S3API
	l   *zerolog.Logger
}

type S3API interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

func New(
	cfg models.Config,
	svc S3API,
	l *zerolog.Logger,
) S3Cleaner {
	return &s3clean{
		cfg: cfg,
		svc: svc,
		l:   l,
	}
}

// Wipes out the entire bucket.  This can be used by itself to empty a bucket
// or before a backup if a clean start backup is required.
func (s *s3clean) WipeS3Bucket() (err error) {
	ctx := context.Background()
	p := s3.NewListObjectsV2Paginator(s.svc, &s3.ListObjectsV2Input{Bucket: aws.String(s.cfg.AWS.S3Bucket)})

	for p.HasMorePages() {
		page, pageErr := p.NextPage(ctx)
		if pageErr != nil {
			s.l.Error().Err(pageErr).Str("bucket", s.cfg.AWS.S3Bucket).Msg(msgListObjectsBucketError)
			return pageErr
		}

		if len(page.Contents) == 0 {
			continue
		}

		objects := make([]s3types.ObjectIdentifier, 0, len(page.Contents))
		for i := range page.Contents {
			if page.Contents[i].Key == nil {
				continue
			}
			objects = append(objects, s3types.ObjectIdentifier{Key: page.Contents[i].Key})
		}

		if len(objects) == 0 {
			continue
		}

		_, deleteErr := s.svc.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.cfg.AWS.S3Bucket),
			Delete: &s3types.Delete{Objects: objects, Quiet: aws.Bool(true)},
		})
		if deleteErr != nil {
			s.l.Error().Err(deleteErr).Str("bucket", s.cfg.AWS.S3Bucket).Msg(msgDeleteObjectsBucketErr)
			return deleteErr
		}
	}

	return nil
}

// This method is intended to be run after a backup but can be run by itself.
// It is used to remove any files in S3 which do not exist in the backup list provided in
// the config file.
func (s *s3clean) SyncS3Bucket() (err error) {

	input := s.createInput()

	result, done := s.objectList(input)
	if done {
		return
	}

	for i := range result.Contents {
		s3file := *result.Contents[i].Key
		osfile := "/" + *result.Contents[i].Key
		_, err = os.Open(osfile)
		if errors.Is(err, os.ErrNotExist) {
			s.l.Info().Str("local_path", osfile).Msg(msgMissingLocalFile)
			err = s.deleteS3File(input, s3file)
			if err != nil {
				s.l.Warn().Err(err).Str("s3_key", s3file).Msg(msgUnableToRemoveS3File)
			} else {
				s.l.Info().Str("local_path", osfile).Str("s3_key", s3file).Msg(msgRemovedFromS3)
			}
		}
	}

	return nil
}

func (s *s3clean) deleteS3File(input *s3.ListObjectsV2Input, s3file string) error {

	deleteInput := &s3.DeleteObjectInput{
		Bucket: input.Bucket,
		Key:    aws.String(s3file),
	}
	_, err := s.svc.DeleteObject(context.Background(), deleteInput)

	if err != nil {
		s.l.Info().Err(err).Msg(msgDeleteObjectFailed)
		return err
	}

	return nil
}

func (s *s3clean) createInput() *s3.ListObjectsV2Input {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.cfg.AWS.S3Bucket),
		MaxKeys: aws.Int32(200),
	}
	return input
}

func (s *s3clean) objectList(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, bool) {
	result, err := s.svc.ListObjectsV2(context.Background(), input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NoSuchBucket" {
				s.l.Info().Err(err).Msg(msgNoSuchBucket)
			} else {
				s.l.Info().Err(err).Msg(msgListObjectsFailed)
			}
		} else {
			s.l.Info().Err(err).Msg(msgListObjectsFailed)
		}
		return nil, true
	}
	return result, false
}
