package main

import (
	"github.com/jaysonhurd/s3backup/s3backup"
	"log"
)

func main() {

	cfg, err := s3backup.LoadConfig("config/config.json")
	if err != nil {
		log.Fatal(err)
	}

	s, err := s3backup.CreateAWSSession(cfg)
	if err != nil {
		log.Fatal(err)
	}

	for i := range cfg.BackupDirs {
		s3backup.BackupDir(cfg, s, cfg.BackupDirs[i])
		if err != nil {
			log.Fatal(err)
		}
	}
}


