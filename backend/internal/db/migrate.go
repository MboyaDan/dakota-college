package db

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/MboyaDan/dakota-college/backend/migrations"
)

// NewMigrator returns a goose provider over the embedded migrations and the
// *sql.DB it uses; the caller closes the DB. Migrations never run on API
// start-up: only the migrate command and tests call this.
func NewMigrator(url string) (*goose.Provider, *sql.DB, error) {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return nil, nil, errors.New("open database: invalid connection string")
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("goose provider: %w", err)
	}
	return provider, sqlDB, nil
}
