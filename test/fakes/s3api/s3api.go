// Package s3api contains a hand-written S3 test double. Each test owns its fake.
package s3api

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type FakeS3API struct {
	Head        func(context.Context, *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)
	Put         func(context.Context, *s3.PutObjectInput) (*s3.PutObjectOutput, error)
	List        func(context.Context, *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
	Delete      func(context.Context, *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
	DeleteBatch func(context.Context, *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error)
}

func (f *FakeS3API) HeadObject(ctx context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return f.Head(ctx, in)
}
func (f *FakeS3API) PutObject(ctx context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return f.Put(ctx, in)
}
func (f *FakeS3API) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return f.List(ctx, in)
}
func (f *FakeS3API) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return f.Delete(ctx, in)
}
func (f *FakeS3API) DeleteObjects(ctx context.Context, in *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	return f.DeleteBatch(ctx, in)
}
