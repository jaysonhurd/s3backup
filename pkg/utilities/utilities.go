package utilities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

func CreateAWSSession(cfg models.Config, l *zerolog.Logger) (aws.Config, error) {
	return CreateAWSConfig(context.Background(), cfg)
}

func CreateAWSConfig(ctx context.Context, cfg models.Config) (aws.Config, error) {
	if (cfg.AWS.AccessKeyId == "") != (cfg.AWS.SecretAccessKey == "") {
		return aws.Config{}, fmt.Errorf("AccessKeyId and SecretAccessKey must be provided together")
	}
	opts := []func(*config.LoadOptions) error{}
	if cfg.AWS.S3Region != "" {
		opts = append(opts, config.WithRegion(cfg.AWS.S3Region))
	}
	if cfg.AWS.AccessKeyId != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AWS.AccessKeyId, cfg.AWS.SecretAccessKey, cfg.AWS.SessionToken)))
	}
	result, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS configuration: %w", err)
	}
	if result.Region == "" {
		return aws.Config{}, fmt.Errorf("AWS region is required in config, environment, or shared AWS profile")
	}
	return result, nil
}

func LoadConfig(filename string) (models.Config, error) {
	var cfg models.Config
	data, err := os.ReadFile(filename)
	if err != nil {
		return cfg, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode configuration: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return cfg, fmt.Errorf("configuration must contain exactly one JSON object")
	}
	return cfg, cfg.Validate()
}

func LoggerSetup(cfg models.Config, level zerolog.Level) (*zerolog.Logger, error) {
	l, _, err := OpenLogger(cfg, level)
	return l, err
}

// OpenLogger returns a closer so rotation files are flushed/closed on normal exit.
func OpenLogger(cfg models.Config, level zerolog.Level) (*zerolog.Logger, io.Closer, error) {
	dir := cfg.Logging.LogfileLocation
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}
	filename := filepath.Join(dir, "s3backup.log")
	// Open now so permission/path errors cannot disappear in the logging writer.
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, nil, err
	}
	rotation := &lumberjack.Logger{Filename: filename, MaxBackups: cfg.Logging.MaxBackups, MaxSize: cfg.Logging.MaxSize, MaxAge: cfg.Logging.MaxAge}
	var writer io.Writer = rotation
	if cfg.Logging.Console {
		writer = zerolog.MultiLevelWriter(rotation, zerolog.ConsoleWriter{Out: os.Stderr})
	}
	l := zerolog.New(writer).Level(level).With().Timestamp().Logger()
	return &l, rotation, nil
}
