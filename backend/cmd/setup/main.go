// Command setup creates the first Super Admin of a new system from
// environment variables (CLAUDE.md: production gets one Super Admin this way).
// It refuses if any Super Admin already exists.
//
//	SUPER_ADMIN_EMAIL=owner@college.example SUPER_ADMIN_NAME="Full Name" \
//	SUPER_ADMIN_PASSWORD='a long passphrase' go run ./cmd/setup
//
// Unset SUPER_ADMIN_PASSWORD afterwards; the person should change it after
// first sign-in.
//
// It also turns off a staff member's two-step sign-in when nobody who could
// reset it in the staff portal can sign in (Stage 17b):
//
//	go run ./cmd/setup reset-two-step person@college.example
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/db"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		os.Exit(1)
	}
}

func run() error {
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

	if len(os.Args) > 1 {
		if os.Args[1] != "reset-two-step" || len(os.Args) != 3 {
			return fmt.Errorf("usage: setup [reset-two-step EMAIL]")
		}
		if err := auth.ResetTwoStepByEmail(ctx, pool, os.Args[2]); err != nil {
			return err
		}
		fmt.Printf("two-step sign-in turned off for %s; they were signed out and can set it up again\n", auth.NormaliseEmail(os.Args[2]))
		return nil
	}

	id, err := auth.CreateFirstSuperAdmin(ctx, pool,
		os.Getenv("SUPER_ADMIN_EMAIL"), os.Getenv("SUPER_ADMIN_NAME"), os.Getenv("SUPER_ADMIN_PASSWORD"))
	if err != nil {
		return err
	}
	fmt.Printf("created Super Admin %s (user %s)\n", auth.NormaliseEmail(os.Getenv("SUPER_ADMIN_EMAIL")), id)
	return nil
}
