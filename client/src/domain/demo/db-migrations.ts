/**
 * Domain models for the databases-sql topic's transactions demo — the
 * client-side shape of `GET /api/demo/db/migrations`. Field names match
 * the server's camelCase wire contract 1:1.
 *
 * Counts are deterministic on the seeded database (6,7,7,8,7,6);
 * `appliedAt` varies per environment by design (when THIS database ran
 * the migration).
 */

/** One row of goose's migration ledger. */
export interface MigrationsHistoryRow {
  version: number;
  applied: boolean;
  appliedAt: string;
}

/** One observed step of the begin/savepoint/rollback dance. */
export interface TxStoryStep {
  step: number;
  action: string;
  sql: string;
  postsVisible: number;
  note: string;
}

export interface MigrationsDemoData {
  history: MigrationsHistoryRow[];
  steps: TxStoryStep[];
  summary: string;
}
