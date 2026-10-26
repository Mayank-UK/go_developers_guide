package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"guide/internal/domain/demos"
)

// Introspector reads the live schema through PostgreSQL's own catalog —
// the REAL pg_catalog/pg_index views, not a copy of the migration files.
// What it returns is what the database says about itself right now, which
// is exactly what the schema-explorer demo promises to show (and what its
// tests assert against: if a migration claim and the catalog ever
// disagree, the catalog wins and the test fails).
type Introspector struct {
	pool *pgxpool.Pool
}

// NewIntrospector wires the introspector over the shared pool.
func NewIntrospector(pool *pgxpool.Pool) *Introspector {
	return &Introspector{pool: pool}
}

// Compile-time proof the adapter satisfies the demo's domain port.
var _ demos.SchemaIntrospector = (*Introspector)(nil)

// Introspect gathers tables, columns, constraints, indexes, and goose's
// migration ledger in five queries, each ordered by the same keys the
// payload builder sorts by — the catalog does the first sort, the builder
// guarantees it.
func (i *Introspector) Introspect(ctx context.Context) (demos.SchemaSnapshot, error) {
	snapshot := demos.SchemaSnapshot{}

	tables, err := i.pool.Query(ctx, `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		ORDER BY c.relname`)
	if err != nil {
		return demos.SchemaSnapshot{}, fmt.Errorf("introspect tables: %w", err)
	}
	defer tables.Close()
	for tables.Next() {
		var name string
		if err := tables.Scan(&name); err != nil {
			return demos.SchemaSnapshot{}, fmt.Errorf("scan table: %w", err)
		}
		snapshot.Tables = append(snapshot.Tables, name)
	}
	if err := tables.Err(); err != nil {
		return demos.SchemaSnapshot{}, fmt.Errorf("iterate tables: %w", err)
	}

	columns, err := i.pool.Query(ctx, `
		SELECT c.relname AS table_name,
		       a.attname AS column_name,
		       format_type(a.atttypid, a.atttypmod) AS data_type,
		       NOT a.attnotnull AS nullable,
		       pg_get_expr(d.adbin, d.adrelid) AS column_default
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		  AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY c.relname, a.attnum`)
	if err != nil {
		return demos.SchemaSnapshot{}, fmt.Errorf("introspect columns: %w", err)
	}
	defer columns.Close()
	for columns.Next() {
		var row demos.ColumnRow
		var defaultValue *string
		if err := columns.Scan(&row.Table, &row.Name, &row.Type, &row.Nullable, &defaultValue); err != nil {
			return demos.SchemaSnapshot{}, fmt.Errorf("scan column: %w", err)
		}
		row.HasDefault = defaultValue != nil
		if defaultValue != nil {
			row.Default = *defaultValue
		}
		snapshot.Columns = append(snapshot.Columns, row)
	}
	if err := columns.Err(); err != nil {
		return demos.SchemaSnapshot{}, fmt.Errorf("iterate columns: %w", err)
	}

	constraints, err := i.pool.Query(ctx, `
		SELECT con.conname AS name,
		       con.conrelid::regclass::text AS table_name,
		       CASE con.contype
		           WHEN 'p' THEN 'PRIMARY KEY'
		           WHEN 'f' THEN 'FOREIGN KEY'
		           WHEN 'u' THEN 'UNIQUE'
		           WHEN 'c' THEN 'CHECK'
		       END AS kind,
		       pg_get_constraintdef(con.oid, true) AS definition
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		ORDER BY con.conrelid::regclass::text, con.conname`)
	if err != nil {
		return demos.SchemaSnapshot{}, fmt.Errorf("introspect constraints: %w", err)
	}
	defer constraints.Close()
	for constraints.Next() {
		var row demos.ConstraintRow
		if err := constraints.Scan(&row.Name, &row.Table, &row.Kind, &row.Definition); err != nil {
			return demos.SchemaSnapshot{}, fmt.Errorf("scan constraint: %w", err)
		}
		if strings.TrimSpace(row.Kind) == "" {
			// contypes outside the four spelled out above (e.g. triggers'
			// identity constraints) report their raw type instead of a blank.
			row.Kind = "OTHER"
		}
		snapshot.Constraints = append(snapshot.Constraints, row)
	}
	if err := constraints.Err(); err != nil {
		return demos.SchemaSnapshot{}, fmt.Errorf("iterate constraints: %w", err)
	}

	indexes, err := i.pool.Query(ctx, `
		SELECT t.relname AS table_name,
		       i.relname AS name,
		       am.amname AS kind,
		       idx.indisunique AS is_unique,
		       idx.indisprimary AS is_primary,
		       pg_get_indexdef(i.oid) AS definition,
		       -- COALESCE: an EXPRESSION index (e.g. the FTS gin over
		       -- to_tsvector(...)) has no plain column positions to name,
		       -- so the aggregate comes back NULL — an index with no
		       -- column list, not a broken scan.
		       (SELECT COALESCE(string_agg(a.attname, ', ' ORDER BY k.ord), '')
		          FROM unnest(idx.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord)
		          JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
		       ) AS columns
		FROM pg_index idx
		JOIN pg_class i ON i.oid = idx.indexrelid
		JOIN pg_class t ON t.oid = idx.indrelid
		JOIN pg_am am ON am.oid = i.relam
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'public'
		ORDER BY t.relname, i.relname`)
	if err != nil {
		return demos.SchemaSnapshot{}, fmt.Errorf("introspect indexes: %w", err)
	}
	defer indexes.Close()
	for indexes.Next() {
		var row demos.IndexRow
		var columns string
		if err := indexes.Scan(&row.Table, &row.Name, &row.Kind, &row.Unique, &row.Primary, &row.Definition, &columns); err != nil {
			return demos.SchemaSnapshot{}, fmt.Errorf("scan index: %w", err)
		}
		for _, column := range strings.Split(columns, ", ") {
			if column = strings.TrimSpace(column); column != "" {
				row.Columns = append(row.Columns, column)
			}
		}
		snapshot.Indexes = append(snapshot.Indexes, row)
	}
	if err := indexes.Err(); err != nil {
		return demos.SchemaSnapshot{}, fmt.Errorf("iterate indexes: %w", err)
	}

	history, err := i.pool.Query(ctx, `
		SELECT version_id, is_applied, tstamp::text
		FROM goose_db_version
		ORDER BY id`)
	if err != nil {
		// The ledger only exists once migrations have run; a database
		// without it reports empty history rather than failing the demo.
		if strings.Contains(err.Error(), "does not exist") {
			return snapshot, nil
		}
		return demos.SchemaSnapshot{}, fmt.Errorf("introspect migration history: %w", err)
	}
	defer history.Close()
	for history.Next() {
		var row demos.MigrationRow
		if err := history.Scan(&row.Version, &row.Applied, &row.AppliedAt); err != nil {
			return demos.SchemaSnapshot{}, fmt.Errorf("scan migration row: %w", err)
		}
		snapshot.History = append(snapshot.History, row)
	}
	if err := history.Err(); err != nil {
		return demos.SchemaSnapshot{}, fmt.Errorf("iterate migration history: %w", err)
	}

	return snapshot, nil
}
