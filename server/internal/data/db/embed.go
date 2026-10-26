// Package db owns the guide's PostgreSQL plumbing: the embedded goose
// migration set, the boot-time migration runner, the shared connection
// pool, and the pg_catalog introspector behind the schema-explorer demo.
// Every other data package (topic 17's posts repository onward) receives a
// *pgxpool.Pool from the composition root — never a DSN, never its own
// connection logic.
package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrations exposes the embedded SQL filesystem to the migration runner
// and to tests that assert on the migration set itself. The files live
// under the "migrations" subdirectory, numbered the way goose discovers
// them (00001_init.sql, 00002_audit_log.sql, …).
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		// Unreachable by construction: the embed directive above guarantees
		// the directory exists whenever this package compiles.
		panic("db: embedded migrations directory missing: " + err.Error())
	}
	return sub
}
