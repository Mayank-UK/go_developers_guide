import type { Result } from '../../core/result';
import type { MigrationsRepository } from './db-migrations-repository';
import type { MigrationsDemoData } from './db-migrations';

/**
 * DB_MIGRATIONS_DEMO_USE_CASE — the container token this demo's use case
 * is registered under.
 */
export const DB_MIGRATIONS_DEMO_USE_CASE = Symbol('GetMigrationsDemoUseCase');

/** GetMigrationsDemoUseCase — loads the transaction story payload. */
export class GetMigrationsDemoUseCase {
  private readonly repository: MigrationsRepository;

  constructor(repository: MigrationsRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<MigrationsDemoData>> {
    return this.repository.get();
  }
}
