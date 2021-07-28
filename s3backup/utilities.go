package s3backup

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jaysonhurd/s3backup/models"
	"io/ioutil"
	"os"
	"path/filepath"
)

func CreateAWSSession(cfg *models.Config) (*session.Session, error) {
	s, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.S3Region),
		Credentials: credentials.NewStaticCredentials(
			cfg.AccessKeyId,
			cfg.SecretAccessKey,
			""),
	})

	return s, err
}


func openFile(fileName string) (string, *os.File, error) {
	fileName, _ = filepath.Abs(fileName)
	file, err := os.Open(fileName)
	if err != nil {
		return "", nil, err
	}
	return fileName, file, nil
}

func LoadConfig (configFile string) (*models.Config, error) {
	var BackupConfig *models.Config
	_, err := os.Stat(configFile)
	if err != nil {
		return BackupConfig, err
	}
	f, err := ioutil.ReadFile(configFile)
	if err != nil {
		return BackupConfig, err
	}
	err = json.Unmarshal(f, &BackupConfig)
	if err != nil {
		return BackupConfig, err
	}
	return BackupConfig, err
}
