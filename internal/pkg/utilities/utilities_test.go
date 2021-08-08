package utilities

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jaysonhurd/s3backup/models"
	"reflect"
	"testing"
)

func TestCreateAWSSession(t *testing.T) {
	type args struct {
		cfg *models.Config
	}

	/*	cfg := &models.Config{
		S3Region : "region",
		S3Bucket : "bucket",
		SecretAccessKey  : "xxxxxxxxxxxxxxx",
		AccessKeyId : "yyyyyyyyyyy",
		BackupDirectories :
			models.DirectoryList{
			"/home/test", "/home/root",
			},
		ACL : "",
		ContentDisposition : "",
		ServerSideEncryption : "",
		StorageClass : "GLACIER",

	}*/
	tests := []struct {
		name    string
		args    args
		want    *session.Session
		wantErr bool
	}{
		//{name: "session1", args: {cfg}, },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateAWSSession(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateAWSSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateAWSSession() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {

	var backupDirectories []string
	backupDirectories = append(backupDirectories, "/home/root/test", "/home/root/Documents")

	config := &models.Config{
		S3Region:             "us-east-2",
		S3Bucket:             "samplebucket",
		SecretAccessKey:      "SSSS",
		AccessKeyId:          "kkkk",
		BackupDirectories:    backupDirectories,
		ACL:                  "private",
		ContentDisposition:   "attachment",
		ServerSideEncryption: "AES256",
		StorageClass:         "STANDARD",
	}

	tests := []struct {
		name       string
		configFile string
		want       *models.Config
		wantErr    bool
	}{
		{name: "GoodSampleFile", configFile: "payloads/configtest.json", want: config, wantErr: false},
		{name: "BadSampleFile", configFile: "payloads/configtest_bad.json", want: config, wantErr: true},
		{name: "MissingSampleFile", configFile: "payloads/filedoesnotexist.json", want: config, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadConfig(tt.configFile)
			if err != nil {
				t.Errorf("LoadConfig() error = %v, wantErr %v\n", err, tt.wantErr)
/*				if errors.Is(err, &fs.PathError{
					Op: "stat",
					Path: "payloads/filedoesnotexist.json",
					Err: syscall.Errno(2),
				}) {
			t.Errorf("LoadConfig() error = %v, wantErr %v\n", err, tt.wantErr)
				}*/
			}
			if got == nil {
				t.Errorf("LoadConfig() error = %v, wantErr %v\n", err, tt.wantErr)
			}
			if got == tt.want {
				t.Errorf("LoadConfig() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got, tt.want) {
				tt.wantErr = false
				t.Errorf("LoadConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}
