import type { Result } from '../../core/result';
import type { SQLLabRepository } from './sql-lab-repository';
import type { SQLLabDemoData } from './sql-lab';

/** DB_SQL_LAB_DEMO_USE_CASE — the container token for this demo's use case. */
export const DB_SQL_LAB_DEMO_USE_CASE = Symbol('GetSQLLabDemoUseCase');

/** GetSQLLabDemoUseCase — loads the SQL lab payload. */
export class GetSQLLabDemoUseCase {
  private readonly repository: SQLLabRepository;

  constructor(repository: SQLLabRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<SQLLabDemoData>> {
    return this.repository.get();
  }
}
