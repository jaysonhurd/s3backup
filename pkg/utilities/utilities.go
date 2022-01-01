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

func CreateAWSSession(cfg *models.Config, l *zerolog.Logger) (*session.Session, error) {
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

func LoadConfig(configFile string) (*models.Config, error) {
	var BackupConfig *models.Config
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

func LoggerSetup(cfg *models.Config, llevel zerolog.Level) (*zerolog.Logger, error) {
	var (
		l zerolog.Logger
		//writers []io.Writer
		err error
	)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(llevel)

	//logFileName := fmt.Sprintf("s3backup_%s%s", time.RFC3339, ".log")
	logFileName := "s3backup.log"
	//writers = append(writers, newRollingFile(cfg, logFileName))
	if cfg.Logging.Console {
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
		multi := zerolog.MultiLevelWriter(consoleWriter, os.Stdout)
		l = zerolog.New(multi).With().Timestamp().Logger()
	}
	//mw := io.MultiWriter(writers...)

	l = zerolog.New(&lumberjack.Logger{
		Filename:   path.Join(cfg.Logging.LogfileLocation, logFileName),
		MaxBackups: cfg.Logging.MaxBackups, // files
		MaxSize:    cfg.Logging.MaxSize,    // megabytes
		MaxAge:     cfg.Logging.MaxAge,     // days
	})

	l = l.With().Caller().Timestamp().Logger()

	return &l, err
}

func PrintHelp() {
	fmt.Printf(`Program Usage:
		-backup : 	Backs up the filesystems listed in config.json
		-config : 	Relative or full path to config file (requires a valid path e.g. '-config configs/config.json'
		-sync 	: 	Reconciles s3 with local filesystem.  Any files not found on the local filesystem
					will be removed from S3
		-wipe 	: 	Wipes the entire S3 bucket from the config.json file
		-force	:	Forces a wipe without asking for confirmation
		`)

}
