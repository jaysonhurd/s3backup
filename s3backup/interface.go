package s3backup

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jaysonhurd/s3backup/models"
)

type S3backup interface {
	BackupDir (cfg *models.Config, s *session.Session, dir string)
}