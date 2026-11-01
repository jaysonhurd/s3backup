package s3clean

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/jaysonhurd/s3backup/internal/logging"
	"github.com/jaysonhurd/s3backup/internal/telemetry"
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

// S3Cleaner defines bucket cleanup operations.
type S3Cleaner interface {
	SyncS3Bucket() (err error)
	WipeS3Bucket() (err error)
}

// s3clean supports s3clean.
type s3clean struct {
	cfg models.Config
	svc S3API
	l   *slog.Logger
}

// S3API defines the S3 operations consumed by this package.
type S3API interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

// New constructs a runner with explicit configuration and S3 dependencies.
func New(
	cfg models.Config,
	svc S3API,
	l *zerolog.Logger,
) S3Cleaner {
	return &s3clean{
		cfg: cfg,
		svc: svc,
		l:   logging.FromLegacy(l),
	}
}

// Wipes out the entire bucket.  This can be used by itself to empty a bucket
// or before a backup if a clean start backup is required.
func (s *s3clean) WipeS3Bucket() (err error) {
	return s.WipeS3BucketContext(context.Background())
}

// WipeS3BucketContext wipes the bucket while honoring cancellation.
func (s *s3clean) WipeS3BucketContext(ctx context.Context) (err error) {
	ctx, finish := telemetry.Start(ctx, "clean.wipe")
	defer func() { finish(err) }()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	p := s3.NewListObjectsV2Paginator(s.svc, &s3.ListObjectsV2Input{Bucket: aws.String(s.cfg.AWS.S3Bucket)})

	for p.HasMorePages() {
		page, pageErr := p.NextPage(ctx)
		if pageErr != nil {
			s.l.Error(msgListObjectsBucketError, slog.Any("error", pageErr), slog.String("bucket", s.cfg.AWS.S3Bucket))
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
			s.l.Error(msgDeleteObjectsBucketErr, slog.Any("error", deleteErr), slog.String("bucket", s.cfg.AWS.S3Bucket))
			return deleteErr
		}
	}

	return nil
}

// This method is intended to be run after a backup but can be run by itself.
// It is used to remove any files in S3 which do not exist in the backup list provided in
// the config file.
func (s *s3clean) SyncS3Bucket() (err error) {
	return s.SyncS3BucketContext(context.Background())
}

// SyncS3BucketContext reconciles S3 objects while honoring cancellation.
func (s *s3clean) SyncS3BucketContext(ctx context.Context) (err error) {
	ctx, finish := telemetry.Start(ctx, "clean.sync")
	defer func() { finish(err) }()
	if ctx.Err() != nil {
		return ctx.Err()
	}

	input := s.createInput()

	result, done := s.objectList(ctx, input)
	if done {
		return ctx.Err()
	}

	for i := range result.Contents {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if result.Contents[i].Key == nil {
			continue
		}
		s3file := *result.Contents[i].Key
		osfile := "/" + *result.Contents[i].Key
		_, err = os.Stat(osfile)
		if errors.Is(err, os.ErrNotExist) {
			s.l.Info(msgMissingLocalFile, slog.String("local_path", osfile))
			err = s.deleteS3File(ctx, input, s3file)
			if err != nil {
				s.l.Warn(msgUnableToRemoveS3File, slog.Any("error", err), slog.String("s3_key", s3file))
			} else {
				s.l.Info(msgRemovedFromS3, slog.String("local_path", osfile), slog.String("s3_key", s3file))
			}
		}
	}

	return nil
}

// deleteS3File supports delete s3 file.
func (s *s3clean) deleteS3File(ctx context.Context, input *s3.ListObjectsV2Input, s3file string) error {

	deleteInput := &s3.DeleteObjectInput{
		Bucket: input.Bucket,
		Key:    aws.String(s3file),
	}
	_, err := s.svc.DeleteObject(ctx, deleteInput)

	if err != nil {
		s.l.Info(msgDeleteObjectFailed, slog.Any("error", err))
		return err
	}

	return nil
}

// createInput supports create input.
func (s *s3clean) createInput() *s3.ListObjectsV2Input {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.cfg.AWS.S3Bucket),
		MaxKeys: aws.Int32(200),
	}
	return input
}

// objectList supports object list.
func (s *s3clean) objectList(ctx context.Context, input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, bool) {
	result, err := s.svc.ListObjectsV2(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NoSuchBucket" {
				s.l.Info(msgNoSuchBucket, slog.Any("error", err))
			} else {
				s.l.Info(msgListObjectsFailed, slog.Any("error", err))
			}
		} else {
			s.l.Info(msgListObjectsFailed, slog.Any("error", err))
		}
		return nil, true
	}
	return result, false
}
