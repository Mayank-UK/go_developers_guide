/**
 * Domain models for the databases-sql topic's SQL lab — the client-side
 * shape of `GET /api/demo/db/sql-lab`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * Rows are flattened display cells: string or number (the gap-less
 * oldest row renders its null as the explicit "—" string cell).
 */

/** One exhibit: its SQL, its teaching line, its columns, its rows. */
export interface SQLLabQuery {
  id: string;
  title: string;
  sql: string;
  teach: string;
  columns: string[];
  rows: (string | number)[][];
}

export interface SQLLabDemoData {
  queries: SQLLabQuery[];
}
