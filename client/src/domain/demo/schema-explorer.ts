/**
 * Domain models for the sql-schema topic's schema-explorer demo — the
 * client-side shape of `GET /api/demo/db/schema-explorer`. Field names
 * match the server's camelCase wire contract 1:1 (the DTO is the seam
 * where wire and domain may diverge later).
 *
 * Everything in this payload is the LIVE catalog's own account of the
 * schema (pg_catalog + pg_get_constraintdef + pg_get_indexdef + goose's
 * ledger), except `purpose` strings and the `notes` list, which are the
 * server's curated rationales for subjects that exist in the snapshot.
 */

/** One introspected column of a table. */
export interface SchemaExplorerColumn {
  /** Column name as declared (declaration order is preserved). */
  name: string;
  /** PostgreSQL's format_type spelling, e.g. `uuid`, `text`,
   * `timestamp with time zone`. */
  type: string;
  /** NOT NULL columns carry false. */
  nullable: boolean;
  /** The default expression; an empty string when the column has none. */
  default: string;
}

/** One introspected table with its curated one-line purpose. */
export interface SchemaExplorerTable {
  name: string;
  /** Why this table exists — the curated half; empty for uncurated
   * tables (present-but-empty is data, not absence). */
  purpose: string;
  columns: SchemaExplorerColumn[];
}

/** One introspected table constraint, definition rendered by PostgreSQL
 * itself (ON DELETE behavior included). */
export interface SchemaExplorerConstraint {
  table: string;
  name: string;
  /** PRIMARY KEY | FOREIGN KEY | UNIQUE | CHECK (| OTHER). */
  kind: string;
  definition: string;
}

/** One introspected index. */
export interface SchemaExplorerIndex {
  table: string;
  name: string;
  /** Access method: btree, gin, … */
  kind: string;
  /** Covered columns, in index order. */
  columns: string[];
  unique: boolean;
  /** Primary-key backing indexes carry true — PKs ARE indexes. */
  primary: boolean;
  definition: string;
}

/** One row of goose's migration ledger. `appliedAt` stamps when THIS
 * database ran the migration — same files, different stamp per
 * environment. */
export interface SchemaExplorerHistory {
  version: number;
  applied: boolean;
  appliedAt: string;
}

/** One curated rationale, keyed to the catalog subject it explains. */
export interface SchemaExplorerNote {
  subject: string;
  note: string;
}

export interface SchemaExplorerDemoData {
  tables: SchemaExplorerTable[];
  constraints: SchemaExplorerConstraint[];
  indexes: SchemaExplorerIndex[];
  history: SchemaExplorerHistory[];
  notes: SchemaExplorerNote[];
}
