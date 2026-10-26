import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { SQLSafetyRepository } from '../../domain/demo/sql-safety-repository';
import type { SQLSafetyDemoData } from '../../domain/demo/sql-safety';
import { toSQLSafetyDemo } from './sql-safety-dto';
import type { SQLSafetyApi } from './sql-safety-api';

/**
 * SQLSafetyRepositoryImpl — adapts the demo API to the domain port; every
 * thrown transport error is guarded into a Failed Result.
 */
export class SQLSafetyRepositoryImpl implements SQLSafetyRepository {
  private readonly api: SQLSafetyApi;

  constructor(api: SQLSafetyApi) {
    this.api = api;
  }

  async get(): Promise<Result<SQLSafetyDemoData>> {
    return safeRepositoryCall(async () => toSQLSafetyDemo(await this.api.get()));
  }
}
