package main

import (
	"github.com/jaysonhurd/s3backup/internal/pkg/utilities"
	"github.com/jaysonhurd/s3backup/pkg/s3backup"
	"github.com/jaysonhurd/s3backup/pkg/s3clean"
	"log"
)

func main() {

	cfg, err := utilities.LoadConfig("config/config.json")
	if err != nil {
		log.Fatal(err)
	}

	sess, err := utilities.CreateAWSSession(cfg)
	if err != nil {
		log.Fatal(err)
	}

	for i := range cfg.BackupDirectories {
		backup := s3backup.CreateBackup(
			*cfg,
			*sess,
			cfg.BackupDirectories[i],
		)
		s3backup.BackupDirectory(backup)
		if err != nil {
			log.Fatal(err)
		}
	}

	bucketToClean := s3clean.CreateClean(*cfg, *sess)
	s3clean.CleanS3Bucket(bucketToClean)

}


