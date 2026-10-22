// Package testdb gives integration tests a real, empty PostgreSQL database.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/db"
)

// URL returns TEST_DATABASE_URL. Without it the test is skipped locally, but
// fails when REQUIRE_DB_TESTS=1 (CI and `make test-db`), so a missing
// database is never reported as a pass.
func URL(t testing.TB) string {
	t.Helper()
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		if os.Getenv("REQUIRE_DB_TESTS") == "1" {
			t.Fatal("TEST_DATABASE_URL is not set but REQUIRE_DB_TESTS=1")
		}
		t.Skip("TEST_DATABASE_URL not set; start Postgres with `make up` to run")
	}
	return u
}

// Fresh creates a new empty database for this test and drops it afterwards.
// It returns the new database's URL. The TEST_DATABASE_URL user needs
// CREATEDB (true for the Compose and CI databases).
func Fresh(t testing.TB) string {
	t.Helper()
	base := URL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect to TEST_DATABASE_URL: %v", err)
	}
	defer admin.Close(ctx)

	var b [6]byte
	_, _ = rand.Read(b[:])
	name := "dakota_t_" + hex.EncodeToString(b[:])
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, base)
		if err != nil {
			t.Logf("drop test database %s: %v", name, err)
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Logf("drop test database %s: %v", name, err)
		}
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

// Migrated creates a fresh database, applies every migration, and returns a
// pool connected to it.
func Migrated(t testing.TB) (*pgxpool.Pool, string) {
	t.Helper()
	u := Fresh(t)
	ctx := context.Background()

	provider, sqlDB, err := db.NewMigrator(u)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	defer sqlDB.Close()
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := db.Open(ctx, u)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, u
}
