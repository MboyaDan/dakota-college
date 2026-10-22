// Package migrations embeds the goose SQL migrations so the migrate command
// and tests use exactly the files in this directory.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
