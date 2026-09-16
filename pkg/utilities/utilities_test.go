package utilities

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
)

func TestCreateAWSSession(t *testing.T) {
	cfg := models.Config{AWS: models.AWS{S3Region: "us-east-1"}}
	l := zerolog.Nop()

	got, err := CreateAWSSession(cfg, &l)
	if err != nil {
		t.Fatalf("CreateAWSSession() unexpected error = %v", err)
	}

	if got.Region != cfg.AWS.S3Region {
		t.Fatalf("CreateAWSSession() region = %q, want %q", got.Region, cfg.AWS.S3Region)
	}
}

func TestLoadConfig(t *testing.T) {
	type args struct {
		configFile string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "missing config", args: args{configFile: "does-not-exist.json"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(tt.args.configFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name, json string
		valid      bool
	}{
		{"valid", `{"AWS":{"S3Bucket":"bucket"},"Logging":{"console":true}}`, true},
		{"flat template", `{"S3Bucket":"bucket","BackupDirs":["/tmp"]}`, false},
		{"unknown", `{"AWS":{"S3Bucket":"bucket","BackupDirs":["/tmp"]}}`, false},
		{"missing bucket", `{"AWS":{}}`, false},
		{"partial credentials", `{"AWS":{"S3Bucket":"bucket","AccessKeyId":"key"}}`, false},
		{"invalid ACL", `{"AWS":{"S3Bucket":"bucket","ACL":"wrong"}}`, false},
		{"invalid storage", `{"AWS":{"S3Bucket":"bucket","StorageClass":"wrong"}}`, false},
		{"KMS key without KMS", `{"AWS":{"S3Bucket":"bucket","SSEKMSKeyID":"key"}}`, false},
		{"trailing JSON", `{"AWS":{"S3Bucket":"bucket"}} {}`, false},
		{"null", `null`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(file, []byte(tc.json), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadConfig(file)
			if (err == nil) != tc.valid {
				t.Fatalf("err=%v valid=%v", err, tc.valid)
			}
		})
	}
}

func TestTemporaryCredentials(t *testing.T) {
	cfg := models.Config{AWS: models.AWS{S3Region: "us-east-1", AccessKeyId: "test", SecretAccessKey: "secret", SessionToken: "session"}}
	got, err := CreateAWSConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := got.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if credentials.SessionToken != "session" {
		t.Fatal("session token lost")
	}
}

func TestLoggerWritesPlainJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := models.Config{Logging: models.Logging{LogfileLocation: dir}}
	l, closer, err := OpenLogger(cfg, zerolog.InfoLevel)
	if err != nil {
		t.Fatal(err)
	}
	l.Info().Msg("test message")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "s3backup.log"))
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("invalid JSON log: %s", data)
	}
	if event["message"] != "test message" {
		t.Fatalf("nested/lost message: %s", data)
	}
}
