package db_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MboyaDan/dakota-college/backend/internal/db"
	"github.com/MboyaDan/dakota-college/backend/internal/testdb"
)

func TestOpenAndPing(t *testing.T) {
	url := testdb.URL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, url)
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
	_, err := db.Open(context.Background(), "postgres://user:s3cret@host:notaport/db")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("error leaks password: %v", err)
	}
}
