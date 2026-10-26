// Topic 16 sql-schema's first live demo: the schema explorer. The demo
// answers "what does the database ACTUALLY say about itself?" — not what
// the migration files claim, but the live catalog truth: tables, columns
// with their real PostgreSQL types, constraints with their definitions,
// indexes with their access-method kinds, and the goose migration history.
// The pgx/pg_catalog work lives in internal/data/db (the introspector
// below); this file owns the port's types and the pure payload builder, so
// the layering stays honest: data → domain, never the reverse.
package demos

import (
	"context"
	"sort"
)

// SchemaIntrospector is the port the schema-explorer demo reads through:
// one call returns the database's self-description as catalog rows. The
// pgx-backed implementation lives in internal/data/db; tests and the
// handler wire fakes through this interface.
type SchemaIntrospector interface {
	// Introspect reads the live schema's tables, columns, constraints,
	// indexes, and migration history from the public schema.
	Introspect(ctx context.Context) (SchemaSnapshot, error)
}

// SchemaSnapshot is the introspector's raw haul: exactly what the catalog
// queries returned, unsorted and uncurated — the builder below shapes it.
type SchemaSnapshot struct {
	// Tables lists every base table in the public schema, goose's version
	// bookkeeping included — it is live truth, not decoration.
	Tables []string
	// Columns is one row per column: which table, which name, the
	// format_type spelling of its type ("uuid", "text",
	// "timestamp with time zone"), nullability, and the human-readable
	// default expression when one exists.
	Columns []ColumnRow
	// Constraints is one row per table constraint with PostgreSQL's own
	// rendering of its definition (pg_get_constraintdef).
	Constraints []ConstraintRow
	// Indexes is one row per index: the columns it covers, its access
	// method (btree, gin, …), uniqueness/primality, and the full
	// pg_get_indexdef statement.
	Indexes []IndexRow
	// History is the goose_db_version ledger: one row per applied
	// migration, in apply order.
	History []MigrationRow
}

// ColumnRow is one introspected column.
type ColumnRow struct {
	Table      string
	Name       string
	Type       string
	Nullable   bool
	HasDefault bool
	Default    string
}

// ConstraintRow is one introspected table constraint.
type ConstraintRow struct {
	Table      string
	Name       string
	Kind       string // PRIMARY KEY | FOREIGN KEY | UNIQUE | CHECK
	Definition string
}

// IndexRow is one introspected index.
type IndexRow struct {
	Table      string
	Name       string
	Kind       string // access method: btree, gin, …
	Columns    []string
	Unique     bool
	Primary    bool
	Definition string
}

// MigrationRow is one applied goose migration, as its own ledger records it.
type MigrationRow struct {
	Version   int64
	Applied   bool
	AppliedAt string
}

// SchemaExplorerTable is one table in the payload: its name, its columns
// in declaration order, and a one-line purpose (the curated half of the
// demo — the live catalog cannot know why a table exists).
type SchemaExplorerTable struct {
	Name    string                 `json:"name"`
	Purpose string                 `json:"purpose"`
	Columns []SchemaExplorerColumn `json:"columns"`
}

// SchemaExplorerColumn is one column as the demo serves it.
type SchemaExplorerColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default"`
}

// SchemaExplorerConstraint is one constraint as the demo serves it.
type SchemaExplorerConstraint struct {
	Table      string `json:"table"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Definition string `json:"definition"`
}

// SchemaExplorerIndex is one index as the demo serves it.
type SchemaExplorerIndex struct {
	Table      string   `json:"table"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Columns    []string `json:"columns"`
	Unique     bool     `json:"unique"`
	Primary    bool     `json:"primary"`
	Definition string   `json:"definition"`
}

// SchemaExplorerHistory is one applied migration from goose's ledger. The
// appliedAt timestamp is when THIS database ran it — same migration set,
// different stamp per environment, so only the version is stable.
type SchemaExplorerHistory struct {
	Version   int64  `json:"version"`
	Applied   bool   `json:"applied"`
	AppliedAt string `json:"appliedAt"`
}

// SchemaExplorerNote is the curated half: why a named subject (table,
// index, constraint) looks the way it does. Notes whose subject is absent
// from the live snapshot are dropped — the demo never explains a thing the
// database cannot show.
type SchemaExplorerNote struct {
	Subject string `json:"subject"`
	Note    string `json:"note"`
}

// SchemaExplorerPayload is the wire shape of GET /api/demo/db/schema-explorer
// (camelCase JSON). Deterministic for a given schema: every collection is
// sorted by name, columns by declaration order, and every slice non-nil so
// the wire renders [] — never null.
type SchemaExplorerPayload struct {
	Tables      []SchemaExplorerTable      `json:"tables"`
	Constraints []SchemaExplorerConstraint `json:"constraints"`
	Indexes     []SchemaExplorerIndex      `json:"indexes"`
	History     []SchemaExplorerHistory    `json:"history"`
	Notes       []SchemaExplorerNote       `json:"notes"`
}

// schemaPurposes and schemaRationales are the demo's curated layer: the
// design decisions the topic teaches, keyed by the catalog name they
// explain. Everything else in the payload is the catalog's own truth.
var schemaPurposes = map[string]string{
	"posts":            "the guide's posts resource — topic 17 swaps the in-memory repository for this table behind the same port",
	"tags":             "the tag vocabulary — natural-keyed by name, one row per distinct tag",
	"post_tags":        "the many-to-many edge between posts and tags",
	"audit_log":        "append-only history of post changes, written inside the changing transaction",
	"goose_db_version": "goose's own ledger — one row per applied migration, the table that makes Up idempotent",
}

var schemaRationales = []SchemaExplorerNote{
	{Subject: "posts_pkey", Note: "primary key over uuid — every PK implicitly creates its supporting btree index."},
	{Subject: "posts_created_at_idx", Note: "explicit btree, DESC: the API's default listing is newest-first, so the index matches the query's shape."},
	{Subject: "post_tags_pkey", Note: "composite PK (post_id, tag_name): identity, no-duplicate-tags, and the posts→tags lookup in one constraint."},
	{Subject: "post_tags_tag_name_idx", Note: "PostgreSQL does not index foreign key columns for you — this btree exists for the reverse lookup: posts carrying a given tag."},
	{Subject: "audit_log_post_id_fkey", Note: "ON DELETE SET NULL: the audit trail outlives its subject — history survives a delete instead of being cascaded away."},
	{Subject: "audit_log_post_changed_idx", Note: "composite btree (post_id, changed_at DESC): one post's history, newest first, without a sort node."},
	{Subject: "tags_name_fkey", Note: "ON DELETE CASCADE: deleting a tag removes its edges, never the posts they pointed at."},
}

// BuildSchemaExplorerPayload shapes a raw snapshot into the demo payload:
// columns grouped under their tables (sorted by table name, columns kept
// in declaration order), constraints and indexes sorted by (table, name),
// history in apply order, and only those curated notes whose subject the
// snapshot actually contains. The snapshot arrives sorted from the catalog
// queries; the builder sorts again defensively, so the payload is
// deterministic even for a hand-built snapshot.
func BuildSchemaExplorerPayload(snapshot SchemaSnapshot) SchemaExplorerPayload {
	sortedTables := append([]string(nil), snapshot.Tables...)
	sort.Strings(sortedTables)

	columnsByTable := make(map[string][]SchemaExplorerColumn)
	for _, column := range snapshot.Columns {
		defaultValue := column.Default
		if !column.HasDefault {
			defaultValue = ""
		}
		columnsByTable[column.Table] = append(columnsByTable[column.Table], SchemaExplorerColumn{
			Name:     column.Name,
			Type:     column.Type,
			Nullable: column.Nullable,
			Default:  defaultValue,
		})
	}

	tables := make([]SchemaExplorerTable, 0, len(sortedTables))
	for _, name := range sortedTables {
		tables = append(tables, SchemaExplorerTable{
			Name:    name,
			Purpose: schemaPurposes[name],
			Columns: nonNilColumns(columnsByTable[name]),
		})
	}

	constraints := make([]SchemaExplorerConstraint, 0, len(snapshot.Constraints))
	for _, constraint := range snapshot.Constraints {
		constraints = append(constraints, SchemaExplorerConstraint{
			Table:      constraint.Table,
			Name:       constraint.Name,
			Kind:       constraint.Kind,
			Definition: constraint.Definition,
		})
	}
	sort.Slice(constraints, func(i, j int) bool {
		if constraints[i].Table != constraints[j].Table {
			return constraints[i].Table < constraints[j].Table
		}
		return constraints[i].Name < constraints[j].Name
	})

	indexes := make([]SchemaExplorerIndex, 0, len(snapshot.Indexes))
	for _, index := range snapshot.Indexes {
		indexes = append(indexes, SchemaExplorerIndex{
			Table:      index.Table,
			Name:       index.Name,
			Kind:       index.Kind,
			Columns:    nonNilStrings(index.Columns),
			Unique:     index.Unique,
			Primary:    index.Primary,
			Definition: index.Definition,
		})
	}
	sort.Slice(indexes, func(i, j int) bool {
		if indexes[i].Table != indexes[j].Table {
			return indexes[i].Table < indexes[j].Table
		}
		return indexes[i].Name < indexes[j].Name
	})

	history := make([]SchemaExplorerHistory, 0, len(snapshot.History))
	for _, migration := range snapshot.History {
		history = append(history, SchemaExplorerHistory{
			Version:   migration.Version,
			Applied:   migration.Applied,
			AppliedAt: migration.AppliedAt,
		})
	}

	known := make(map[string]bool, len(constraints)+len(indexes)+len(tables))
	for _, table := range tables {
		known[table.Name] = true
	}
	for _, constraint := range constraints {
		known[constraint.Name] = true
	}
	for _, index := range indexes {
		known[index.Name] = true
	}
	notes := make([]SchemaExplorerNote, 0, len(schemaRationales))
	for _, note := range schemaRationales {
		if known[note.Subject] {
			notes = append(notes, note)
		}
	}

	return SchemaExplorerPayload{
		Tables:      tables,
		Constraints: constraints,
		Indexes:     indexes,
		History:     history,
		Notes:       notes,
	}
}

// nonNilColumns keeps a table's column list non-nil even for a table whose
// columns did not arrive (defense for hand-built snapshots).
func nonNilColumns(columns []SchemaExplorerColumn) []SchemaExplorerColumn {
	if columns == nil {
		return []SchemaExplorerColumn{}
	}
	return columns
}

// nonNilStrings materializes nil as [] for the wire's never-null rule.
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
