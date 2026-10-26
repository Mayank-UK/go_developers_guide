/**
 * Domain models for the databases-sql topic's full-text search demo —
 * the client-side shape of `GET /api/demo/db/search`. Field names match
 * the server's camelCase wire contract 1:1.
 *
 * Deterministic on the seeded database: the stemmed tsquery, the three
 * ranked matches, and the chosen plan are stable per boot.
 */

/** One ranked hit. */
export interface SearchMatch {
  title: string;
  /** ts_rank rendered as a fixed 4-decimal string. */
  rank: string;
}

export interface SearchDemoData {
  query: string;
  tsQuery: string;
  sql: string;
  matches: SearchMatch[];
  /** The EXPLAIN (estimate-only) plan lines. */
  plan: string[];
  /** pg_get_indexdef's rendering of the GIN expression index. */
  indexDefinition: string;
  teach: string;
}
