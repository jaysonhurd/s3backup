package s3clean

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jaysonhurd/s3backup/models"
)

type s3cleanData struct {
	cfg models.Config
	awsSess session.Session
}

type S3clean interface {
	Clean()
}

// Constructor for cleaning up an S3 Bucket
func CreateClean (cfg models.Config, sess session.Session) *s3cleanData {
	return &s3cleanData{
		cfg: cfg,
		awsSess: sess,
	}
}

