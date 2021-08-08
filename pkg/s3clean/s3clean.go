package s3clean

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"os"
)

func CleanS3Bucket(s *s3cleanData) {

	svc := s3.New(&s.awsSess)
	input := createInput(s)

	result, done := getObjectList(svc, input)
	if done {
		return
	}

	for i := range result.Contents {
		s3file := *result.Contents[i].Key
		osfile := "/" + *result.Contents[i].Key
		_, err := os.Open(osfile)
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println(osfile, " does not exist locally. Removing from S3")
			deleteS3File(input, s3file, svc)
		}
	}

	return
}

func deleteS3File(input *s3.ListObjectsV2Input, s3file string, svc *s3.S3) error {

	deleteInput := &s3.DeleteObjectInput{
		Bucket: input.Bucket,
		Key:    &s3file,
	}
	_, err := svc.DeleteObject(deleteInput)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			fmt.Println(err.Error())
		}
		return err
	}

	return nil
}

func createInput(s *s3cleanData) *s3.ListObjectsV2Input {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.cfg.S3Bucket),
		MaxKeys: aws.Int64(200),
	}
	return input
}

func getObjectList(svc *s3.S3, input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, bool) {
	result, err := svc.ListObjectsV2(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				fmt.Println(s3.ErrCodeNoSuchBucket, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			fmt.Println(err.Error())
		}
		return nil, true
	}
	return result, false
}
