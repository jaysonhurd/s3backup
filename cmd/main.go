package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jaysonhurd/s3backup/models"
	"github.com/jaysonhurd/s3backup/pkg/s3backup"
	"github.com/jaysonhurd/s3backup/pkg/s3clean"
	"github.com/jaysonhurd/s3backup/pkg/utilities"
	"github.com/rs/zerolog"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "s3backup:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("s3backup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configFile := flags.String("config", "/etc/config.json", "path to the configuration file")
	backup := flags.Bool("backup", false, "back up configured directories")
	sync := flags.Bool("sync", false, "delete missing local files from configured S3 prefixes")
	wipe := flags.Bool("wipe", false, "delete all current objects in the bucket (retains object versions)")
	force := flags.Bool("force", false, "skip wipe confirmation")
	dryRun := flags.Bool("dry-run", false, "report planned uploads/deletions without changing S3")
	level := flags.String("llevel", "info", "logging level: trace, debug, info, warn, error, fatal, panic, disabled")
	console := flags.Bool("console", false, "also log to stderr")
	help := flags.Bool("help", false, "show usage")
	showVersion := flags.Bool("version", false, "show build version")
	timeout := flags.Duration("timeout", 0, "overall operation timeout (e.g. 30m); zero disables")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *help {
		flags.PrintDefaults()
		return nil
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return nil
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *force && !*wipe {
		return fmt.Errorf("-force requires -wipe")
	}
	if *timeout < 0 {
		return fmt.Errorf("-timeout cannot be negative")
	}
	logLevel, err := zerolog.ParseLevel(*level)
	if err != nil {
		return err
	}
	if !*backup && !*sync && !*wipe {
		flags.PrintDefaults()
		return nil
	}
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*timeout))
		defer cancel()
	}
	cfg, err := utilities.LoadConfig(*configFile)
	if err != nil {
		return err
	}
	cfg.DryRun = *dryRun
	cfg.Logging.Console = cfg.Logging.Console || *console || *dryRun
	if *backup || *sync {
		cfg.AWS.BackupDirectories, err = models.BackupRoots(cfg.AWS.BackupDirectories)
		if err != nil {
			return err
		}
	}
	l, closer, err := utilities.OpenLogger(cfg, logLevel)
	if err != nil {
		return err
	}
	defer closer.Close()
	awsCfg, err := utilities.CreateAWSConfig(ctx, cfg)
	if err != nil {
		return err
	}
	svc := s3.NewFromConfig(awsCfg)
	cleaner := s3clean.NewWithContext(ctx, cfg, svc, l)
	if *wipe {
		if !*force && !*dryRun {
			fmt.Fprintf(stderr, "Delete all current objects in bucket %q? Type the bucket name to confirm: ", cfg.AWS.S3Bucket)
			answer, err := readAnswer(ctx, stdin)
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			if strings.TrimSpace(answer) != cfg.AWS.S3Bucket {
				return fmt.Errorf("wipe cancelled")
			}
		}
		if err := cleaner.WipeS3Bucket(); err != nil {
			return err
		}
		l.Info().Bool("dry_run", cfg.DryRun).Msg("wipe completed; historical versions and delete markers are retained")
	}
	if *backup {
		for _, dir := range cfg.AWS.BackupDirectories {
			if err := s3backup.NewWithContext(ctx, cfg, svc, dir, l).BackupDirectory(); err != nil {
				return err
			}
		}
	}
	if *sync {
		if err := cleaner.SyncS3Bucket(); err != nil {
			return err
		}
	}
	return nil
}

// Keep cancellation responsive while waiting for interactive confirmation.
func readAnswer(ctx context.Context, in io.Reader) (string, error) {
	type response struct {
		answer string
		err    error
	}
	ready := make(chan response, 1)
	go func() {
		answer, err := bufio.NewReader(in).ReadString('\n')
		ready <- response{answer, err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-ready:
		return result.answer, result.err
	}
}
