import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { SQLLabRepository } from '../../domain/demo/sql-lab-repository';
import type { SQLLabDemoData } from '../../domain/demo/sql-lab';
import { toSQLLabDemo } from './sql-lab-dto';
import type { SQLLabApi } from './sql-lab-api';

/** SQLLabRepositoryImpl — adapts the demo API to the domain port. */
export class SQLLabRepositoryImpl implements SQLLabRepository {
  private readonly api: SQLLabApi;

  constructor(api: SQLLabApi) {
    this.api = api;
  }

  async get(): Promise<Result<SQLLabDemoData>> {
    return safeRepositoryCall(async () => toSQLLabDemo(await this.api.get()));
  }
}
