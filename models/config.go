package models

import (
	"sync"
)

// Config holds application configuration.
type Config struct {
	AWS     AWS     `json:"AWS"`
	Logging Logging `json:"Logging"`
}

// Logging controls rotating log output.
type Logging struct {
	LogfileLocation string `json:"logfile_location"`
	MaxBackups      int    `json:"max_backups"`
	MaxSize         int    `json:"max_size"`
	MaxAge          int    `json:"max_age"`
	Console         bool   `json:"Console"`
}

// AWS holds S3 settings and optional static credentials.
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

// AppConfig stores configuration behind a mutex for future coordinated access.
type AppConfig struct {
	mut    *sync.Mutex
	config *Config
}

// NewAppConfig supports new app config.
func NewAppConfig(config *Config) *AppConfig {
	return &AppConfig{
		mut:    new(sync.Mutex),
		config: config,
	}
}
