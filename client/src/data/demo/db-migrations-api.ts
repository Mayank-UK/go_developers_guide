import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { migrationsDtoFromJson, type MigrationsDemoDto } from './db-migrations-dto';

/** MigrationsApi — the transport for GET /api/demo/db/migrations. */

const DEMO_PATH = '/demo/db/migrations';

export class MigrationsApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/migrations — the tx story + goose ledger. */
  async get(): Promise<MigrationsDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, migrationsDtoFromJson);
    });
  }
}
