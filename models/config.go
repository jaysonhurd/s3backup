package models

type Config struct {
	S3Region             string        `json:"S3Region"`
	S3Bucket             string        `json:"S3Bucket"`
	SecretAccessKey      string        `json:"SecretAccessKey"`
	AccessKeyId          string        `json:"AccessKeyId"`
	BackupDirectories    []string `json:"BackupDirectories"`
	ACL                  string        `json:"ACL"`
	ContentDisposition   string        `json:"ContentDisposition"`
	ServerSideEncryption string        `json:"ServerSideEncryption"`
	StorageClass         string        `json:"StorageClass"`
}


