package utilities

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
)

// TestLoadConfigCompatibility exercises success and malformed configuration paths.
func TestLoadConfigCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		wantErr     bool
	}{
		{"legacy lowercase logging", `{"AWS":{"S3Bucket":"test","BackupDirectories":["/data"]},"logging":{"logfile_location":"/tmp/logs","Console":true}}`, false},
		{"malformed JSON", `{"AWS":`, true},
		{"wrong field type", `{"AWS":{"S3Bucket":42}}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(file, []byte(tt.input), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfig(file)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadConfig error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (cfg.AWS.S3Bucket != "test" || len(cfg.AWS.BackupDirectories) != 1 || !cfg.Logging.Console || cfg.Logging.LogfileLocation != "/tmp/logs") {
				t.Fatalf("legacy configuration changed: %+v", cfg)
			}
		})
	}
	if _, err := LoadConfig(t.TempDir()); err == nil {
		t.Fatal("reading directory must fail")
	}
}

// TestLoggerSetup checks configured destinations and filesystem errors.
func TestLoggerSetup(t *testing.T) {
	oldLevel := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(oldLevel) })
	dir := t.TempDir()
	cfg := models.Config{Logging: models.Logging{LogfileLocation: filepath.Join(dir, "nested"), Console: true}}
	logger, err := LoggerSetup(cfg, zerolog.InfoLevel)
	if err != nil {
		t.Fatal(err)
	}
	logger.Info().Msg("test log")
	if _, err := os.Stat(filepath.Join(cfg.Logging.LogfileLocation, "s3backup.log")); err != nil {
		t.Fatal(err)
	}
	cfg.Logging.LogfileLocation = filepath.Join(dir, "file")
	if err := os.WriteFile(cfg.Logging.LogfileLocation, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoggerSetup(cfg, zerolog.InfoLevel); err == nil {
		t.Fatal("logger directory error was ignored")
	}
}

// TestConfigTemplate ensures the distributed example matches the actual schema.
func TestConfigTemplate(t *testing.T) {
	cfg, err := LoadConfig("../../config/config.json.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AWS.S3Bucket == "" || len(cfg.AWS.BackupDirectories) == 0 || cfg.Logging.LogfileLocation != "logs" {
		t.Fatalf("incomplete configuration template: %+v", cfg)
	}
}
