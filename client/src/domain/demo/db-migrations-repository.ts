import type { Result } from '../../core/result';
import type { MigrationsDemoData } from './db-migrations';

/**
 * MigrationsRepository — the domain-facing port for the transactions
 * demo payload (the savepoint story plus goose's ledger).
 */
export interface MigrationsRepository {
  /** The live transaction story with the goose ledger beside it. */
  get(): Promise<Result<MigrationsDemoData>>;
}
