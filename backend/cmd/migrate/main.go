// Command migrate applies the embedded database migrations.
//
// Usage:
//
//	migrate status       list migrations and whether each is applied
//	migrate up           apply all pending migrations
//	migrate down         roll back the most recent migration
//	migrate reset        roll back everything (refused in production)
//
// Migrations never run automatically. In production, `up` and `down` also
// need the flag, placed before the command:
//
//	APP_ENV=production migrate -confirm-production up
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pressly/goose/v3"

	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/db"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	confirmProduction := fs.Bool("confirm-production", false, "required to change a production database")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("expected one command: status, up, down or reset")
	}
	command := fs.Arg(0)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := checkAllowed(cfg.Env, command, *confirmProduction); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	provider, sqlDB, err := db.NewMigrator(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	switch command {
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			state := "pending"
			if s.State == goose.StateApplied {
				state = "applied " + s.AppliedAt.Format("2006-01-02 15:04")
			}
			fmt.Printf("%05d  %-40s %s\n", s.Source.Version, s.Source.Path, state)
		}
	case "up":
		return report(provider.Up(ctx))
	case "down":
		res, err := provider.Down(ctx)
		if err != nil {
			return err
		}
		return report([]*goose.MigrationResult{res}, nil)
	case "reset":
		return report(provider.DownTo(ctx, 0))
	default:
		return fmt.Errorf("unknown command %q", command)
	}
	return nil
}

// checkAllowed guards production: reset is never allowed, and any change
// needs an explicit flag. Status is always safe.
func checkAllowed(env config.Env, command string, confirmProduction bool) error {
	if env != config.EnvProduction || command == "status" {
		return nil
	}
	if command == "reset" {
		return fmt.Errorf("reset is not allowed in production")
	}
	if !confirmProduction {
		return fmt.Errorf("APP_ENV=production: to %s the production database, run: migrate -confirm-production %s", command, command)
	}
	return nil
}

func report(results []*goose.MigrationResult, err error) error {
	for _, r := range results {
		if r == nil {
			continue
		}
		fmt.Printf("%s %05d %s (%s)\n", r.Direction, r.Source.Version, r.Source.Path, r.Duration.Round(1e6))
	}
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Println("nothing to do")
	}
	return nil
}
