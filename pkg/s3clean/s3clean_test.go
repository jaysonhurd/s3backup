package s3clean_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/jaysonhurd/s3backup/pkg/s3clean"
	"github.com/jaysonhurd/s3backup/test/fakes/s3api"
	"github.com/rs/zerolog"
)

// TestWipeBucket supports test wipe bucket.
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

// TestSyncS3Bucket supports test sync s3 bucket.
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

// TestWipeFailures verifies list and delete failures are returned unchanged.
func TestWipeFailures(t *testing.T) {
	for _, stage := range []string{"list", "delete"} {
		t.Run(stage, func(t *testing.T) {
			fake := new(s3api.FakeS3API)
			failure := errors.New("request failed")
			fake.ListObjectsV2Returns(&s3.ListObjectsV2Output{Contents: []types.Object{{Key: aws.String("file")}}}, nil)
			if stage == "list" {
				fake.ListObjectsV2Returns(nil, failure)
			} else {
				fake.DeleteObjectsReturns(nil, failure)
			}
			cleaner := s3clean.New(models.Config{}, fake, nil)
			if err := cleaner.WipeS3Bucket(); !errors.Is(err, failure) {
				t.Fatalf("error = %v, want %v", err, failure)
			}
		})
	}
}

// TestWipeEmptyAndNilKeys verifies nondeletable listings are harmless.
func TestWipeEmptyAndNilKeys(t *testing.T) {
	for _, contents := range [][]types.Object{nil, {{Key: nil}}} {
		fake := new(s3api.FakeS3API)
		fake.ListObjectsV2Returns(&s3.ListObjectsV2Output{Contents: contents}, nil)
		if err := s3clean.New(models.Config{}, fake, nil).WipeS3Bucket(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCleanupCancellation verifies optional context methods before any S3 work.
func TestCleanupCancellation(t *testing.T) {
	cleaner := s3clean.New(models.Config{}, nil, nil)
	contextual := cleaner.(interface {
		WipeS3BucketContext(context.Context) error
		SyncS3BucketContext(context.Context) error
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := contextual.WipeS3BucketContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wipe error = %v", err)
	}
	if err := contextual.SyncS3BucketContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("sync error = %v", err)
	}
}
