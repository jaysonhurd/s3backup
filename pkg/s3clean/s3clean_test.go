package s3clean_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/jaysonhurd/s3backup/pkg/s3clean"
	"github.com/jaysonhurd/s3backup/test/fakes/s3api"
	"github.com/rs/zerolog"
)

func TestWipeBucket(t *testing.T) {
	l := zerolog.Nop()
	cfg := models.Config{AWS: models.AWS{S3Bucket: "test-bucket"}}

	fake := new(s3api.FakeS3API)
	fake.ListObjectsV2Returns(&s3.ListObjectsV2Output{
		Contents: []types.Object{{Key: aws.String("old/file.txt")}},
	}, nil)
	fake.DeleteObjectsReturns(&s3.DeleteObjectsOutput{}, nil)

	cleaner := s3clean.New(cfg, fake, &l)
	if err := cleaner.WipeS3Bucket(); err != nil {
		t.Fatalf("WipeS3Bucket() unexpected error: %v", err)
	}
}

func TestSyncS3Bucket(t *testing.T) {
	l := zerolog.Nop()
	cfg := models.Config{AWS: models.AWS{S3Bucket: "test-bucket"}}

	fake := new(s3api.FakeS3API)
	fake.ListObjectsV2Returns(&s3.ListObjectsV2Output{
		Contents: []types.Object{{Key: aws.String("missing-local.txt")}},
	}, nil)
	fake.DeleteObjectReturns(&s3.DeleteObjectOutput{}, nil)

	cleaner := s3clean.New(cfg, fake, &l)
	if err := cleaner.SyncS3Bucket(); err != nil {
		t.Fatalf("SyncS3Bucket() unexpected error: %v", err)
	}
}
