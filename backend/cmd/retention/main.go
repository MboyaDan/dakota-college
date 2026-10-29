// Command retention runs the data retention schedule once (Stage 17c,
// docs/retention.md) and prints what it found.
//
//	go run ./cmd/retention          # report: count what is due, erase nothing
//	go run ./cmd/retention apply    # erase what is past its period
//
// Housekeeping (spent sign-in links, ended sessions) is removed in both
// modes. The API runs the same schedule daily in RETENTION_MODE; use this
// to see a report before turning apply on, or to apply it by hand.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/db"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
	"github.com/MboyaDan/dakota-college/backend/internal/privacy"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "retention:", err)
		os.Exit(1)
	}
}

func run() error {
	mode := privacy.ModeReport
	switch {
	case len(os.Args) == 1:
	case len(os.Args) == 2 && (os.Args[1] == privacy.ModeReport || os.Args[1] == privacy.ModeApply):
		mode = os.Args[1]
	default:
		return fmt.Errorf("usage: retention [report|apply]")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	var store media.Storage
	if cfg.S3.Enabled() {
		s3, err := media.NewS3(media.S3Config{Endpoint: cfg.S3.Endpoint, Region: cfg.S3.Region, AccessKey: cfg.S3.AccessKey, SecretKey: cfg.S3.SecretKey, UseSSL: cfg.S3.UseSSL})
		if err != nil {
			return err
		}
		store = s3
	} else {
		fmt.Fprintln(os.Stderr, "object storage not configured: deleted files stay in storage until a run with S3 settings")
	}

	res, err := privacy.NewService(privacy.Options{Pool: pool, Storage: store, Mode: mode}).Run(ctx, mode)
	if res != nil {
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "RULE\tDUE\tREMOVED\n")
		for _, r := range res.Results {
			fmt.Fprintf(w, "%s\t%d\t%d\n", r.Rule, r.Due, r.Done)
		}
		_ = w.Flush()
		fmt.Printf("run %s, mode %s\n", res.ID, res.Mode)
	}
	return err
}
