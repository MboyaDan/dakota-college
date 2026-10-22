// Command seed fills a development or test database with fake sample data.
// It refuses to run unless APP_ENV is development or test, and refuses hosted
// database hosts (see seed.Guard). Production never gets seed data (AC-17).
//
// Needs SEED_PASSWORD (at least 12 characters): the password for every test
// account it creates, such as admin@example.invalid.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/db"
	"github.com/MboyaDan/dakota-college/backend/seed"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := seed.Guard(cfg.Env, cfg.DatabaseURL); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	sum, err := seed.Run(ctx, pool, os.Getenv("SEED_PASSWORD"))
	if err != nil {
		return err
	}
	fmt.Printf("seeded sample data: %d schools, %d programmes, %d intakes, %d fee items, %d FAQs, %d settings, %d test users\n",
		sum.Schools, sum.Programmes, sum.Intakes, sum.FeeItems, sum.FAQs, sum.Settings, sum.Users)
	fmt.Println("test accounts use SEED_PASSWORD; Super Admin is admin@example.invalid")
	return nil
}
