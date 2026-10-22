package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/db"
	"github.com/MboyaDan/dakota-college/backend/internal/testdb"
	"github.com/MboyaDan/dakota-college/backend/migrations"
)

// Stage 3 done-when: migrations apply from empty, roll back fully, and roll
// forward again cleanly.
func TestMigrationsUpDownUp(t *testing.T) {
	url := testdb.Fresh(t)
	ctx := context.Background()

	provider, sqlDB, err := db.NewMigrator(url)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	files, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var sqlFiles int
	for _, f := range files {
		if !f.IsDir() && len(f.Name()) > 4 && f.Name()[len(f.Name())-4:] == ".sql" {
			sqlFiles++
		}
	}

	up, err := provider.Up(ctx)
	if err != nil {
		t.Fatalf("first up: %v", err)
	}
	if len(up) != sqlFiles {
		t.Fatalf("applied %d migrations, want %d", len(up), sqlFiles)
	}
	if n := countTables(t, url); n < 30 {
		t.Fatalf("only %d tables after up", n)
	}

	down, err := provider.DownTo(ctx, 0)
	if err != nil {
		t.Fatalf("down to 0: %v", err)
	}
	if len(down) != sqlFiles {
		t.Fatalf("rolled back %d migrations, want %d", len(down), sqlFiles)
	}
	// Only goose's own version table may remain.
	if n := countTables(t, url); n != 1 {
		t.Fatalf("%d tables left after full rollback, want 1 (goose_db_version)", n)
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("second up: %v", err)
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != int64(sqlFiles) {
		t.Fatalf("version %d after re-apply, want %d", version, sqlFiles)
	}
	if again, err := provider.Up(ctx); err != nil || len(again) != 0 {
		t.Fatalf("up on an up-to-date database applied %d migrations, err %v", len(again), err)
	}
}

func countTables(t *testing.T, url string) int {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var n int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
