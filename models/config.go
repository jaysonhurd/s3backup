package models

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Config struct {
	AWS     AWS     `json:"AWS"`
	Logging Logging `json:"Logging"`
	DryRun  bool    `json:"-"`
}

type Logging struct {
	LogfileLocation string `json:"logfile_location"`
	MaxBackups      int    `json:"max_backups"`
	MaxSize         int    `json:"max_size"`
	MaxAge          int    `json:"max_age"`
	Console         bool   `json:"console"`
}

type AWS struct {
	S3Region             string   `json:"S3Region"`
	S3Bucket             string   `json:"S3Bucket"`
	SecretAccessKey      string   `json:"SecretAccessKey"`
	AccessKeyId          string   `json:"AccessKeyId"`
	SessionToken         string   `json:"SessionToken"`
	BackupDirectories    []string `json:"BackupDirectories"`
	ACL                  string   `json:"ACL"`
	ContentDisposition   string   `json:"ContentDisposition"`
	ServerSideEncryption string   `json:"ServerSideEncryption"`
	SSEKMSKeyID          string   `json:"SSEKMSKeyID"`
	StorageClass         string   `json:"StorageClass"`
}

// Validate rejects invalid options before any destructive operation can run.
func (c Config) Validate() error {
	if strings.TrimSpace(c.AWS.S3Bucket) == "" {
		return fmt.Errorf("AWS.S3Bucket is required")
	}
	if (c.AWS.AccessKeyId == "") != (c.AWS.SecretAccessKey == "") {
		return fmt.Errorf("AccessKeyId and SecretAccessKey must be provided together")
	}
	if c.AWS.SessionToken != "" && c.AWS.AccessKeyId == "" {
		return fmt.Errorf("SessionToken requires static access keys")
	}
	if c.AWS.ACL != "" && !slices.Contains(types.ObjectCannedACL("").Values(), types.ObjectCannedACL(c.AWS.ACL)) {
		return fmt.Errorf("invalid ACL %q", c.AWS.ACL)
	}
	if c.AWS.StorageClass != "" && !slices.Contains(types.StorageClass("").Values(), types.StorageClass(c.AWS.StorageClass)) {
		return fmt.Errorf("invalid StorageClass %q", c.AWS.StorageClass)
	}
	if c.AWS.ServerSideEncryption != "" && !slices.Contains(types.ServerSideEncryption("").Values(), types.ServerSideEncryption(c.AWS.ServerSideEncryption)) {
		return fmt.Errorf("invalid ServerSideEncryption %q", c.AWS.ServerSideEncryption)
	}
	if c.AWS.SSEKMSKeyID != "" && c.AWS.ServerSideEncryption != "aws:kms" && c.AWS.ServerSideEncryption != "aws:kms:dsse" {
		return fmt.Errorf("SSEKMSKeyID requires aws:kms or aws:kms:dsse encryption")
	}
	if c.Logging.MaxAge < 0 || c.Logging.MaxSize < 0 || c.Logging.MaxBackups < 0 {
		return fmt.Errorf("log rotation settings cannot be negative")
	}
	return nil
}

// BackupRoots resolves paths once so HEAD, PUT, and sync share the same keys.
func BackupRoots(dirs []string) ([]string, error) {
	if len(dirs) == 0 {
		return nil, fmt.Errorf("AWS.BackupDirectories must contain at least one directory")
	}
	roots := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			return nil, fmt.Errorf("backup directory cannot be empty")
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("backup root %q: %w", abs, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("backup root %q is not a directory", abs)
		}
		if !slices.Contains(roots, abs) {
			roots = append(roots, abs)
		}
	}
	// Exclude nested roots to avoid uploading/listing the same files twice.
	slices.Sort(roots)
	result := make([]string, 0, len(roots))
	for _, root := range roots {
		nested := false
		for _, parent := range result {
			rel, err := filepath.Rel(parent, root)
			if err == nil && filepath.IsLocal(rel) {
				nested = true
				break
			}
		}
		if !nested {
			result = append(result, root)
		}
	}
	return result, nil
}
