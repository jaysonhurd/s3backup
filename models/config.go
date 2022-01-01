package models

import (
	"sync"
)

type Config struct {
	AWS     AWS     `json:"AWS"`
	Logging logging `json:"logging"`
}

type logging struct {
	LogfileLocation string `json:"logfile_location"`
	MaxBackups      int    `json:"max_backups"`
	MaxSize         int    `json:"max_size"`
	MaxAge          int    `json:"max_age"`
	Console         bool
}

type AWS struct {
	S3Region             string   `json:"S3Region"`
	S3Bucket             string   `json:"S3Bucket"`
	SecretAccessKey      string   `json:"SecretAccessKey"`
	AccessKeyId          string   `json:"AccessKeyId"`
	BackupDirectories    []string `json:"BackupDirectories"`
	ACL                  string   `json:"ACL"`
	ContentDisposition   string   `json:"ContentDisposition"`
	ServerSideEncryption string   `json:"ServerSideEncryption"`
	StorageClass         string   `json:"StorageClass"`
}

type AppConfig struct {
	mut    *sync.Mutex
	config *Config
}

func NewAppConfig(config *Config) *AppConfig {
	return &AppConfig{
		mut:    new(sync.Mutex),
		config: config,
	}
}
