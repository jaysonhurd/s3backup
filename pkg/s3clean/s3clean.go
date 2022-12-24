package s3clean

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
	"os"
)

type S3Clean interface {
	SyncS3Bucket()
	WipeS3Bucket()
}

type s3clean struct {
	cfg models.Config
	svc s3iface.S3API
	l   *zerolog.Logger
}

// Constructor for cleaning up an S3 Bucket
func New(cfg models.Config, svc s3iface.S3API, l *zerolog.Logger) s3clean {
	return s3clean{
		cfg: cfg,
		svc: svc,
		l:   l,
	}
}

// Wipes out the entire bucket.  This can be used by itself to empty a bucket
// or before a backup if a clean start backup is required.
func (s *s3clean) WipeS3Bucket() {
	//svc := s3.New(&s.awsSess)
	iter := s3manager.NewDeleteListIterator(s.svc, &s3.ListObjectsInput{
		Bucket: &s.cfg.AWS.S3Bucket,
	})
	if err := s3manager.NewBatchDeleteWithClient(s.svc).Delete(aws.BackgroundContext(), iter); err != nil {
		s.l.Fatal().Err(err)
		s.exitErrorf("Unable to delete objects from bucket %q, %v", s.cfg.AWS.S3Bucket, err)
	}
}

// This method is intended to be run after a backup but can be run by itself.
// It is used to remove any files in S3 which do not exist in the backup list provided in
// the config file.
func (s *s3clean) SyncS3Bucket() {

	input := s.createInput()

	result, done := s.getObjectList(input)
	if done {
		return
	}

	for i := range result.Contents {
		s3file := *result.Contents[i].Key
		osfile := "/" + *result.Contents[i].Key
		_, err := os.Open(osfile)
		if errors.Is(err, os.ErrNotExist) {
			s.l.Info().Msgf("%s does not exist locally. Removing from S3", osfile)
			s.deleteS3File(input, s3file)
			if err != nil {
				s.l.Warn().Msgf("%s not found in S3 ", s3file)
			}
		}
	}

	return
}

func (s *s3clean) deleteS3File(input *s3.ListObjectsV2Input, s3file string) error {

	deleteInput := &s3.DeleteObjectInput{
		Bucket: input.Bucket,
		Key:    &s3file,
	}
	_, err := s.svc.DeleteObject(deleteInput)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				s.l.Info().Msg(aerr.Error())
			}
		} else {
			s.l.Info().Msg(aerr.Error())
		}
		return err
	}

	return nil
}

func (s *s3clean) createInput() *s3.ListObjectsV2Input {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.cfg.AWS.S3Bucket),
		MaxKeys: aws.Int64(200),
	}
	return input
}

func (s *s3clean) getObjectList(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, bool) {
	result, err := s.svc.ListObjectsV2(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				s.l.Info().Msg(s3.ErrCodeNoSuchBucket)
			default:
				s.l.Info().Msg(aerr.Error())
			}
		} else {
			s.l.Info().Msg(aerr.Error())
		}
		return nil, true
	}
	return result, false
}

func (s *s3clean) exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"", args...)
	os.Exit(1)
}
