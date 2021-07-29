package s3backup

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jaysonhurd/s3backup/models"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func BackupDir (cfg *models.Config, s *session.Session, dir string)  {

	tmpDir, err := prepareTestDirTree(dir)
	if err != nil {
		log.Fatal(err)
		return
	}
	//defer os.RemoveAll(tmpDir)
	os.Chdir(tmpDir)

 	err = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
			return err
		}
		s3objectTime, err := getS3HeadObjectTimestamp(cfg, s, path)
		if err != nil {
			log.Fatal(err)
		}
		var s3objectTime2 = *s3objectTime

		localFileTime := getLocalFileTimestamp(path)

		if localFileTime.UnixMicro() > s3objectTime2.UnixMicro() {
			fmt.Printf("Backing up file: %q to AWS S3 storage type %s\n", path, cfg.StorageClass)
			err = uploadFileToS3(cfg, s, path)
		} else {
			fmt.Printf("SKIPPING File: %q is same date or older than the version in S3\n", path)
		}

		if err != nil {
			log.Fatal(err)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", tmpDir, err)
		return
	}
	return
}

func getLocalFileTimestamp(file string) time.Time {
	filestat, err := os.Stat(file)
	if err != nil {
		log.Fatal(err)
	}

	return filestat.ModTime()
}

func getS3HeadObjectTimestamp (cfg *models.Config, s *session.Session, file string) (*time.Time, error) {

	svc :=s3.New(s)

	input := &s3.HeadObjectInput{
		Bucket: aws.String(cfg.S3Bucket),
		Key:    aws.String(file),
	}

	result, err := svc.HeadObject(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				//location, _ := time.LoadLocation("US/Denver")
				epoch := time.Date(2000, 01, 01, 01, 01, 01, 1, time.UTC)
				return &epoch, nil
			default:
				log.Fatal(err)
			}
		} else {
			return result.LastModified, nil
		}
		//return result.LastModified, nil
	}
	return result.LastModified, nil
}

func uploadFileToS3(cfg *models.Config, s *session.Session, fileName string) error {

	fileName, file, err := openFile(fileName)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	fileInfo, _ := file.Stat()
	var size = fileInfo.Size()
	buffer := make([]byte, size)
	file.Read(buffer)

	_, s3err := s3.New(s).PutObject(&s3.PutObjectInput{
		Bucket:               aws.String(cfg.S3Bucket),
		Key:                  aws.String(fileName),
		ACL:                  aws.String(cfg.ACL),
		Body:                 bytes.NewReader(buffer),
		ContentLength:        aws.Int64(size),
		ContentType:          aws.String(http.DetectContentType(buffer)),
		ContentDisposition:   aws.String(cfg.ContentDisposition),
		ServerSideEncryption: aws.String(cfg.ServerSideEncryption),
		StorageClass:         aws.String(cfg.StorageClass),
	})

	return s3err

}


func prepareTestDirTree(tree string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %v\n", err)
	}

	err = os.MkdirAll(filepath.Join(tmpDir, tree), 0755)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return tmpDir, nil
}

