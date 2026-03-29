package utilities

import (
	"context"
	"encoding/json"
	"os"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	msgLoadAWSConfigFailed = "unable to load AWS configuration"
	msgProgramUsageHelp    = `Program Usage:
		-backup : 	Backs up the filesystems listed in config.json (default is false)
		-config : 	Relative or full path to config file (requires a valid path e.g. '-config configs/config.json'
		-sync 	: 	Reconciles s3 with local filesystem.  Any files not found on the local filesystem
					will be removed from S3 (default is false)
		-wipe 	: 	Wipes the entire S3 bucket from the config.json file (Default is false)
		-force	:	Forces a wipe without asking for confirmation (Default is false)
		-level  :   Which logging level - Info, Warn, Error, Debug (Default is Error)
		-console :  If you would also like to log to console in addition to the logfile. Default is off (false)
		`
)

func CreateAWSSession(cfg models.Config, l *zerolog.Logger) (aws.Config, error) {
	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.AWS.S3Region),
	}

	if cfg.AWS.AccessKeyId != "" && cfg.AWS.SecretAccessKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AWS.AccessKeyId, cfg.AWS.SecretAccessKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		if l != nil {
			l.Error().Err(err).Msg(msgLoadAWSConfigFailed)
		}
		return aws.Config{}, err
	}

	return awsCfg, nil
}

func LoadConfig(configFile string) (models.Config, error) {
	var BackupConfig models.Config
	_, err := os.Stat(configFile)
	if err != nil {
		return BackupConfig, err
	}
	f, err := os.ReadFile(configFile)
	if err != nil {
		return BackupConfig, err
	}
	err = json.Unmarshal(f, &BackupConfig)
	if err != nil {
		return BackupConfig, err
	}
	return BackupConfig, err
}

func LoggerSetup(cfg models.Config, llevel zerolog.Level) (*zerolog.Logger, error) {
	var (
		l   zerolog.Logger
		err error
	)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(llevel)

	logFileName := "s3backup.log"

	lfile := zerolog.New(&lumberjack.Logger{
		Filename:   path.Join(cfg.Logging.LogfileLocation, logFileName),
		MaxBackups: cfg.Logging.MaxBackups, // files
		MaxSize:    cfg.Logging.MaxSize,    // megabytes
		MaxAge:     cfg.Logging.MaxAge,     // days
	})

	if cfg.Logging.Console {
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
		multi := zerolog.MultiLevelWriter(consoleWriter, lfile)
		l = zerolog.New(multi).With().Timestamp().Logger()
	} else {
		l = zerolog.New(lfile).With().Caller().Timestamp().Logger()
	}

	return &l, err
}

func PrintHelp() {
	log.Info().Msg(msgProgramUsageHelp)
}
