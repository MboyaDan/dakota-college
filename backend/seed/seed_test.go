package seed_test

import (
	"context"
	"testing"

	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/testdb"
	"github.com/MboyaDan/dakota-college/backend/seed"
)

const localURL = "postgres://dakota:x@localhost:5442/dakota?sslmode=disable"

func TestGuardRefusesOutsideDevelopmentAndTest(t *testing.T) {
	for _, env := range []config.Env{config.EnvProduction, config.EnvStaging} {
		if err := seed.Guard(env, localURL); err == nil {
			t.Errorf("Guard(%s) allowed seeding", env)
		}
	}
	for _, env := range []config.Env{config.EnvDevelopment, config.EnvTest} {
		if err := seed.Guard(env, localURL); err != nil {
			t.Errorf("Guard(%s) = %v, want allowed", env, err)
		}
	}
}

func TestGuardRefusesHostedDatabases(t *testing.T) {
	for _, u := range []string{
		"postgres://u:p@db.abcdefgh.supabase.co:5432/postgres",
		"postgres://u:p@aws-0-eu-central-1.pooler.supabase.com:6543/postgres",
		"postgres://u:p@dpg-xyz.frankfurt-postgres.render.com/db",
		"not a url",
	} {
		if err := seed.Guard(config.EnvDevelopment, u); err == nil {
			t.Errorf("Guard allowed seeding %q", u)
		}
	}
}

func TestRunIsIdempotentAndMarksEverythingSample(t *testing.T) {
	pool, _ := testdb.Migrated(t)
	ctx := context.Background()
	const password = "test-only-password-123"

	first, err := seed.Run(ctx, pool, password)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := seed.Run(ctx, pool, password)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if first != second {
		t.Fatalf("runs differ: %+v then %+v", first, second)
	}

	counts := map[string]int{}
	for _, table := range []string{"schools", "programmes", "programme_intakes", "fee_items", "faqs", "users", "user_roles", "accreditations"} {
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		counts[table] = n
	}
	want := map[string]int{
		"schools": 5, "programmes": 8, "programme_intakes": 16, "fee_items": 24,
		"faqs": 5, "users": 10, "user_roles": 10, "accreditations": 1,
	}
	for table, n := range want {
		if counts[table] != n {
			t.Errorf("%s has %d rows after two runs, want %d", table, counts[table], n)
		}
	}

	q := dbq.New(pool)
	samples, err := q.CountSampleRows(ctx)
	if err != nil || samples == 0 {
		t.Fatalf("CountSampleRows = %d, %v; seed rows must be flagged is_sample", samples, err)
	}
	var unflagged int
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM schools WHERE NOT is_sample) + (SELECT count(*) FROM programmes WHERE NOT is_sample)
		     + (SELECT count(*) FROM faqs WHERE NOT is_sample) + (SELECT count(*) FROM fee_items WHERE NOT is_sample)`).Scan(&unflagged); err != nil {
		t.Fatal(err)
	}
	if unflagged != 0 {
		t.Fatalf("%d seeded content rows are not flagged is_sample", unflagged)
	}

	// Every sample programme title says so.
	var unlabelled int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM programmes WHERE title NOT LIKE '%(Sample)'`).Scan(&unlabelled); err != nil || unlabelled != 0 {
		t.Fatalf("%d programmes without (Sample) in the title (%v)", unlabelled, err)
	}

	// Pool Management stays empty for the college to fill (DQ-02).
	var poolProgrammes int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM programmes p JOIN schools s ON s.id = p.school_id WHERE s.slug = 'pool-management'`).Scan(&poolProgrammes); err != nil || poolProgrammes != 0 {
		t.Fatalf("pool management has %d programmes (%v), want 0", poolProgrammes, err)
	}

	published, err := q.ListPublishedProgrammes(ctx)
	if err != nil || len(published) != 7 {
		t.Fatalf("published programmes = %d (%v), want 7 (one is a draft)", len(published), err)
	}

	// Test emails can never reach a real inbox.
	var realEmails int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE email NOT LIKE '%@example.invalid'`).Scan(&realEmails); err != nil || realEmails != 0 {
		t.Fatalf("%d seeded users without an @example.invalid email (%v)", realEmails, err)
	}

	// The test Super Admin can log in with the seed password and holds system.admin.
	var adminID [16]byte
	var hash string
	if err := pool.QueryRow(ctx, `SELECT id, password_hash FROM users WHERE email = 'admin@example.invalid'`).Scan(&adminID, &hash); err != nil {
		t.Fatal(err)
	}
	if ok, err := auth.VerifyPassword(password, hash); err != nil || !ok {
		t.Fatalf("seed password does not verify: %v %v", ok, err)
	}
	perms, err := q.ListUserPermissionKeys(ctx, adminID)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(perms, "system.admin") || len(perms) != 23 {
		t.Fatalf("super admin permissions = %v", perms)
	}
}

func TestRunRejectsShortPassword(t *testing.T) {
	pool, _ := testdb.Migrated(t)
	if _, err := seed.Run(context.Background(), pool, "short"); err == nil {
		t.Fatal("seed accepted a short password")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
