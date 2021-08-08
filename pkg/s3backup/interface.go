package s3backup

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jaysonhurd/s3backup/models"
)

type backupData struct {
	cfg models.Config
	awsSess session.Session
	dir string
}


// Constructor for backing up a single directory
func CreateBackup (cfg models.Config, sess session.Session, dir string) *backupData {
	return &backupData{
		cfg: cfg,
		awsSess:  sess,
		dir: dir,
	}
}

type S3backup interface {
	Backup (cfg *models.Config, sess *session.Session, dir string)
}

func (b backupData) Config() models.Config{
	return b.cfg
}

func (b backupData) AWSSession() session.Session{
return b.awsSess
}

func (b backupData) Directory() string {
return b.dir
}

//TODO: implement validation
func (b *backupData) SetConfig(cfg models.Config) {
	b.cfg = cfg
}

//TODO: implement validation
func (b *backupData) SetAWSSession(sess session.Session) {
	b.awsSess = sess
}

//TODO: implement validation
func (b *backupData) SetDirectory(dir string) {
	b.dir = dir
}