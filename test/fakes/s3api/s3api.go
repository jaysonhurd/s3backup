package s3api

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// FakeS3API is a minimal test double for the subset of S3 APIs used by this project.
type FakeS3API struct {
	headObjectOutput   *s3.HeadObjectOutput
	headObjectErr      error
	putObjectOutput    *s3.PutObjectOutput
	putObjectErr       error
	LastPutObjectInput *s3.PutObjectInput
	listObjectsOutput  *s3.ListObjectsV2Output
	listObjectsErr     error
	deleteObjectOutput *s3.DeleteObjectOutput
	deleteObjectErr    error
	deleteObjsOutput   *s3.DeleteObjectsOutput
	deleteObjsErr      error
}

func (f *FakeS3API) HeadObjectReturns(out *s3.HeadObjectOutput, err error) {
	f.headObjectOutput = out
	f.headObjectErr = err
}
func (f *FakeS3API) PutObjectReturns(out *s3.PutObjectOutput, err error) {
	f.putObjectOutput = out
	f.putObjectErr = err
}
func (f *FakeS3API) ListObjectsV2Returns(out *s3.ListObjectsV2Output, err error) {
	f.listObjectsOutput = out
	f.listObjectsErr = err
}
func (f *FakeS3API) DeleteObjectReturns(out *s3.DeleteObjectOutput, err error) {
	f.deleteObjectOutput = out
	f.deleteObjectErr = err
}
func (f *FakeS3API) DeleteObjectsReturns(out *s3.DeleteObjectsOutput, err error) {
	f.deleteObjsOutput = out
	f.deleteObjsErr = err
}
func (f *FakeS3API) HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if f.headObjectOutput == nil {
		f.headObjectOutput = &s3.HeadObjectOutput{}
	}
	return f.headObjectOutput, f.headObjectErr
}
func (f *FakeS3API) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.LastPutObjectInput = in
	if f.putObjectOutput == nil {
		f.putObjectOutput = &s3.PutObjectOutput{}
	}
	return f.putObjectOutput, f.putObjectErr
}
func (f *FakeS3API) ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if f.listObjectsOutput == nil {
		f.listObjectsOutput = &s3.ListObjectsV2Output{}
	}
	return f.listObjectsOutput, f.listObjectsErr
}
func (f *FakeS3API) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if f.deleteObjectOutput == nil {
		f.deleteObjectOutput = &s3.DeleteObjectOutput{}
	}
	return f.deleteObjectOutput, f.deleteObjectErr
}
func (f *FakeS3API) DeleteObjects(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	if f.deleteObjsOutput == nil {
		f.deleteObjsOutput = &s3.DeleteObjectsOutput{}
	}
	return f.deleteObjsOutput, f.deleteObjsErr
}
