import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { sqlSafetyDtoFromJson, type SQLSafetyDemoDto } from './sql-safety-dto';

/**
 * SQLSafetyApi — the transport for GET /api/demo/db/sql-safety: the
 * guarded fetch pipeline turns transport failures into typed
 * AppExceptions (the server's honest 503 for an unreachable PostgreSQL
 * included), then the JSON is validated into a DTO.
 */

const DEMO_PATH = '/demo/db/sql-safety';

export class SQLSafetyApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/sql-safety — the injection exhibit. */
  async get(): Promise<SQLSafetyDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, sqlSafetyDtoFromJson);
    });
  }
}
