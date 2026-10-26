import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { MigrationsRepository } from '../../domain/demo/db-migrations-repository';
import type { MigrationsDemoData } from '../../domain/demo/db-migrations';
import { toMigrationsDemo } from './db-migrations-dto';
import type { MigrationsApi } from './db-migrations-api';

/** MigrationsRepositoryImpl — adapts the demo API to the domain port. */
export class MigrationsRepositoryImpl implements MigrationsRepository {
  private readonly api: MigrationsApi;

  constructor(api: MigrationsApi) {
    this.api = api;
  }

  async get(): Promise<Result<MigrationsDemoData>> {
    return safeRepositoryCall(async () => toMigrationsDemo(await this.api.get()));
  }
}
