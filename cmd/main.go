package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jaysonhurd/s3backup/pkg/s3backup"
	"github.com/jaysonhurd/s3backup/pkg/s3clean"
	"github.com/jaysonhurd/s3backup/pkg/utilities"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

//TODO: Write tests (centralized fakes for each package)
//TODO: Clean up flag branching - hard to read
//TODO: Write parallel option using wait groups and a goroutine for each directory structure given
//TODO: Write README.md
//TODO: Add a goroutine to continue checking for changed files and backing them up if they change
//TODO: Restore option
//TODO: Create RPM package for distribution
//TODO: Create .deb package for distribution

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

		// Errors and canned messages
		ErrBadLoggerLevel           = errors.New("Invalid logger format!  Options are: Info, Warn, Error, Fatal")
		ErrInvalidWipeResponse      = errors.New("Exiting program, invalid response!")
		ErrNoBackupRequested        = errors.New("You have requested to not back up.  If you wish to back up, please re-run either without the -clean or with the -backup flag")
		InfoWipeNotSelected         = errors.New("Wipe option not selected, bucket is will not be wiped prior to any backup")
		ErrUnableToCreateAWSSession = errors.New("Unable to create AWS session")
		WarnSyncNotSelected         = errors.New("Sync not selected.  Any files located on S3 but not on the local filesystem will not be removed from S3")
		ErrIssueWithBackupDir       = errors.New("Issue with backup directory found")

		// Misc vars
		logLevel zerolog.Level
		err      error

		sess *session.Session
		svc  *s3.S3
	)

	// Begin setup items
	flag.Parse()
	if *fhelp {
		utilities.PrintHelp()
	}
	// Set up logging with zerolog
	if *llevel == "" {
		logLevel = zerolog.ErrorLevel
	}
	if *llevel != "" {
		logLevel, err = zerolog.ParseLevel(*llevel)
		if err != nil {
			fmt.Printf(ErrBadLoggerLevel.Error())
		}
	}

	cfg, err := utilities.LoadConfig(*fconfig)
	if err != nil {
		log.Fatal().Err(err)
		os.Exit(0)
	}

	if *fconsole {
		cfg.Logging.Console = *fconsole
	} else {
		*fconsole = *fconsole
	}

	l, err := utilities.LoggerSetup(cfg, logLevel)

	sess, err = utilities.CreateAWSSession(cfg, l)
	if err != nil {
		l.Fatal().Msg(ErrUnableToCreateAWSSession.Error())
	}
	svc = s3.New(sess)

	// Begin backup procedures
	if *fwipe {
		if !*fforce {
			fmt.Printf("You have selected the -wipe option. THIS WILL WIPE OUT ALL FILES IN YOUR BUCKET named %s in AWS Region %s!!  Do you wish to proceed?", cfg.AWS.S3Bucket, cfg.AWS.S3Region)
			fmt.Printf("Do you wish to continue? [y/n]")
			var answer string
			fmt.Scanln(&answer)
			if answer == "" || answer != "y" {
				l.Fatal().Msg(ErrInvalidWipeResponse.Error())
				os.Exit(1)
			}
		}
		bucketToWipe := s3clean.New(
			cfg,
			svc,
			l,
		)
		bucketToWipe.WipeS3Bucket()
		l.Info().Msgf("Bucket %s has been wiped from S3!", cfg.AWS.S3Bucket)

		if !*fbackup {
			l.Fatal().Msg(ErrNoBackupRequested.Error())
			os.Exit(1)
		}

	} else {
		l.Warn().Msg(InfoWipeNotSelected.Error())
	}

	if *fbackup {
		for i := range cfg.AWS.BackupDirectories {
			backup := s3backup.New(
				cfg,
				svc,
				cfg.AWS.BackupDirectories[i],
				l,
			)
			backup.BackupDirectory()
			if err != nil {
				l.Fatal().Msg(ErrIssueWithBackupDir.Error())
			}
		}
	}

	if *fsync {
		if *fsync {
			cleanBucket := s3clean.New(
				cfg,
				svc,
				l,
			)
			err = cleanBucket.SyncS3Bucket()
			if err != nil {
				l.Error().Msgf("syncBucket failed on bucket %v with error %v", cfg.AWS.S3Bucket, err.Error())
			}
		}
	} else {
		l.Warn().Msg(WarnSyncNotSelected.Error())
	}
}
