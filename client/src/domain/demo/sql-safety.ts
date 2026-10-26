/**
 * Domain models for the databases-sql topic's sql-safety demo — the
 * client-side shape of `GET /api/demo/db/sql-safety`. Field names match
 * the server's camelCase wire contract 1:1.
 *
 * Deterministic on the seeded database: every attack matches zero rows,
 * the post count stays 6, and the verdicts never move.
 */

/** One injection attack: the live parameterized outcome beside the
 * never-executed concatenation exhibit. */
export interface SQLSafetyAttackRow {
  input: string;
  intent: string;
  parameterizedSql: string;
  rowsMatched: number;
  postsAfter: number;
  assembledSql: string;
  assembledEffect: string;
  verdict: string;
}

export interface SQLSafetyDemoData {
  attacks: SQLSafetyAttackRow[];
  summary: string;
}
