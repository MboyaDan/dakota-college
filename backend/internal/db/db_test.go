package db

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// testDatabaseURL returns TEST_DATABASE_URL. Without it the test is skipped
// locally, but fails when REQUIRE_DB_TESTS=1 (set in CI) so a missing
// database is never reported as a pass.
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		if os.Getenv("REQUIRE_DB_TESTS") == "1" {
			t.Fatal("TEST_DATABASE_URL is not set but REQUIRE_DB_TESTS=1")
		}
		t.Skip("TEST_DATABASE_URL not set; start Postgres with `make up` to run")
	}
	return url
}

func TestOpenAndPing(t *testing.T) {
	url := testDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	var one int
	if err := pool.QueryRow(ctx, "select 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("select 1 = %d, %v", one, err)
	}
}

func TestOpenRejectsBadURLWithoutLeakingIt(t *testing.T) {
	_, err := Open(context.Background(), "postgres://user:s3cret@host:notaport/db")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("error leaks password: %v", err)
	}
}
