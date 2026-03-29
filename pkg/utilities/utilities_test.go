package utilities

import (
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
