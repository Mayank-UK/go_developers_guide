import type { Result } from '../../core/result';
import type { SQLSafetyRepository } from './sql-safety-repository';
import type { SQLSafetyDemoData } from './sql-safety';

/**
 * DB_SQL_SAFETY_DEMO_USE_CASE — the container token this demo's use case
 * is registered under; the composition root registers the real pipeline
 * under this exact symbol.
 */
export const DB_SQL_SAFETY_DEMO_USE_CASE = Symbol('GetSQLSafetyDemoUseCase');

/**
 * GetSQLSafetyDemoUseCase — loads the injection exhibit: the canonical
 * attacks executed as $1 parameters against the live posts table.
 */
export class GetSQLSafetyDemoUseCase {
  private readonly repository: SQLSafetyRepository;

  constructor(repository: SQLSafetyRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<SQLSafetyDemoData>> {
    return this.repository.get();
  }
}
