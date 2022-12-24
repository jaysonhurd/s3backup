package utilities

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
	"io/ioutil"
	"os"
	"path"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Utilities

func CreateAWSSession(cfg models.Config, l *zerolog.Logger) (*session.Session, error) {
	s, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.AWS.S3Region),
		Credentials: credentials.NewStaticCredentials(
			cfg.AWS.AccessKeyId,
			cfg.AWS.SecretAccessKey,
			""),
	})

	if err != nil {
		l.Fatal().Err(err)
	}

	return s, err
}

func LoadConfig(configFile string) (models.Config, error) {
	var BackupConfig models.Config
	_, err := os.Stat(configFile)
	if err != nil {
		return BackupConfig, err
	}
	f, err := ioutil.ReadFile(configFile)
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
	fmt.Printf(`Program Usage:
		-backup : 	Backs up the filesystems listed in config.json (default is false)
		-config : 	Relative or full path to config file (requires a valid path e.g. '-config configs/config.json'
		-sync 	: 	Reconciles s3 with local filesystem.  Any files not found on the local filesystem
					will be removed from S3 (default is false)
		-wipe 	: 	Wipes the entire S3 bucket from the config.json file (Default is false)
		-force	:	Forces a wipe without asking for confirmation (Default is false)
		-level  :   Which logging level - Info, Warn, Error, Debug (Default is Error)
		-console :  If you would also like to log to console in addition to the logfile. Default is off (false)
		`)

}
