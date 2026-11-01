package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jaysonhurd/s3backup/internal/logging"
	"github.com/jaysonhurd/s3backup/pkg/s3backup"
	"github.com/jaysonhurd/s3backup/pkg/s3clean"
	"github.com/jaysonhurd/s3backup/pkg/utilities"
	"github.com/rs/zerolog"
)

const (
	msgLoggerSetupFailed     = "Unable to initialize logger"
	msgInvalidLoggerLevel    = "Invalid logger format! Options are: info, warn, error, fatal"
	msgLoadConfigFailed      = "Failed to load config file"
	msgCreateAWSConfigFailed = "Unable to create AWS config"
	msgWipeWarning           = "You have selected the -wipe option. THIS WILL WIPE OUT ALL FILES IN YOUR BUCKET."
	msgWipeContinuePrompt    = "Do you wish to continue? [y/n]"
	msgProgramExiting        = "Exiting program"
	msgInvalidWipeResponse   = "Exiting program, invalid response!"
	msgBucketWiped           = "Bucket has been wiped from S3"
	msgNoBackupRequested     = "You have requested to not back up. If you wish to back up, please re-run either without the -clean or with the -backup flag"
	msgWipeNotSelected       = "Wipe option not selected, bucket will not be wiped prior to any backup"
	msgBackupDirectoryIssue  = "Issue with backup directory found"
	msgSyncBucketFailed      = "syncBucket failed"
	msgSyncNotSelected       = "Sync not selected. Any files located on S3 but not on the local filesystem will not be removed from S3"
)

//TODO: Write tests (centralized fakes for each package)
//TODO: Clean up flag branching - hard to read
//TODO: Write parallel option using wait groups and a goroutine for each directory structure given
//TODO: Write README.md
//TODO: Add a goroutine to continue checking for changed files and backing them up if they change
//TODO: Restore option
//TODO: Create RPM package for distribution
//TODO: Create .deb package for distribution

// main configures dependencies and runs the requested S3 operations.
func main() {

	var (
		// Flags
		fconfig  = flag.String("config", "/etc/config.json", "path to the configuration file")
		fsync    = flag.Bool("sync", false, "After backup, perform a sync by removing files in S3 which no longer exist on the local filesystem")
		fbackup  = flag.Bool("backup", false, "Use this value if doing a fwipe (-clean) if you want to also start a fresh new backup")
		fwipe    = flag.Bool("wipe", false, "Wipe the bucket clean entirely - this is destructive")
		fhelp    = flag.Bool("help", false, "Provide help file info")
		fforce   = flag.Bool("force", false, "Force a wipe without asking for confirmation. Caution!!")
		llevel   = flag.String("llevel", "info", "Logging level - default is info")
		fconsole = flag.Bool("console", false, "Use this flag to also log at console level")
		//background = flag.Bool("background", false, "Runs in the background to check for any changed file, then uploads")

		// Misc vars
		logLevel zerolog.Level
		err      error

		awsCfg aws.Config
		svc    *s3.Client
	)

	// Begin setup items
	flag.Parse()
	if *fhelp {
		utilities.PrintHelp()
		return
	}
	// Retain legacy logger configuration at public API boundaries.
	if *llevel == "" {
		logLevel = zerolog.ErrorLevel
	}
	if *llevel != "" {
		logLevel, err = zerolog.ParseLevel(*llevel)
		if err != nil {
			slog.Default().Error(msgInvalidLoggerLevel, slog.Any("error", err))
		}
	}

	cfg, err := utilities.LoadConfig(*fconfig)
	if err != nil {
		fatal(slog.Default(), msgLoadConfigFailed, slog.Any("error", err))
	}

	if *fconsole {
		cfg.Logging.Console = *fconsole
	}

	l, err := utilities.LoggerSetup(cfg, logLevel)
	if err != nil {
		fatal(slog.Default(), msgLoggerSetupFailed, slog.Any("error", err))
	}

	awsCfg, err = utilities.CreateAWSSession(cfg, l)
	if err != nil {
		fatal(logging.FromLegacy(l), msgCreateAWSConfigFailed, slog.Any("error", err))
	}
	svc = s3.NewFromConfig(awsCfg)

	// Begin backup procedures
	if *fwipe {
		if !*fforce {
			logging.FromLegacy(l).Warn(msgWipeWarning, slog.String("bucket", cfg.AWS.S3Bucket), slog.String("region", cfg.AWS.S3Region))
			logging.FromLegacy(l).Warn(msgWipeContinuePrompt)
			var answer string
			_, err = fmt.Scanln(&answer)
			if err != nil {
				fatal(slog.Default(), msgProgramExiting, slog.Any("error", err))
			}
			if answer != "y" {
				fatal(logging.FromLegacy(l), msgInvalidWipeResponse)
			}
		}
		bucketToWipe := s3clean.New(
			cfg,
			svc,
			l,
		)
		err = bucketToWipe.WipeS3Bucket()
		if err != nil {
			fatal(slog.Default(), msgProgramExiting, slog.Any("error", err))
		}
		logging.FromLegacy(l).Info(msgBucketWiped, slog.String("bucket", cfg.AWS.S3Bucket))

		if !*fbackup {
			fatal(logging.FromLegacy(l), msgNoBackupRequested)
		}

	} else {
		logging.FromLegacy(l).Warn(msgWipeNotSelected)
	}

	if *fbackup {
		for i := range cfg.AWS.BackupDirectories {
			backup := s3backup.New(
				cfg,
				svc,
				cfg.AWS.BackupDirectories[i],
				l,
			)
			err = backup.BackupDirectory()
			if err != nil {
				fatal(logging.FromLegacy(l), msgBackupDirectoryIssue, slog.Any("error", err))
			}
		}
	}

	if *fsync {
		cleanBucket := s3clean.New(
			cfg,
			svc,
			l,
		)
		err = cleanBucket.SyncS3Bucket()
		if err != nil {
			logging.FromLegacy(l).Error(msgSyncBucketFailed, slog.Any("error", err), slog.String("bucket", cfg.AWS.S3Bucket))
		}
	} else {
		logging.FromLegacy(l).Warn(msgSyncNotSelected)
	}
}

// fatal logs a startup or command failure and exits unsuccessfully.
func fatal(logger *slog.Logger, message string, attrs ...any) {
	logger.Log(context.Background(), logging.LevelFatal, message, attrs...)
	os.Exit(1)
}
