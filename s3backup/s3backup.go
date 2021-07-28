package s3backup

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jaysonhurd/s3backup/models"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func BackupDir (cfg *models.Config, s *session.Session, dir string)  {

	tmpDir, err := prepareTestDirTree(dir)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer os.RemoveAll(tmpDir)
	os.Chdir(tmpDir)

 	err = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
			return err
		}
		fmt.Printf("Backing up file: %q to AWS S3 storage type %s\n", path, cfg.StorageClass)
		err = uploadFileToS3(cfg, s, path)
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


func getS3HeadObjectTimestamp (cfg *models.Config, s *session.Session, file string) bool {

	input := &s3.HeadObjectInput{
		Bucket: aws.String(cfg.S3Bucket),
		Key:    aws.String(file),
	}
	result, err := s  .HeadObject(input)
	return true
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

