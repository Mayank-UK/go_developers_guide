import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { sqlLabDtoFromJson, type SQLLabDemoDto } from './sql-lab-dto';

/** SQLLabApi — the transport for GET /api/demo/db/sql-lab. */

const DEMO_PATH = '/demo/db/sql-lab';

export class SQLLabApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/sql-lab — the three exhibits. */
  async get(): Promise<SQLLabDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, sqlLabDtoFromJson);
    });
  }
}
