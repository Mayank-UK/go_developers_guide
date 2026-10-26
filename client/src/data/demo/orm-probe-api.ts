import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { ormProbeDtoFromJson, type ORMProbeDemoDto } from './orm-probe-dto';

/**
 * ORMProbeApi — the transport for GET /api/demo/db/orm-probe, in the
 * same style as data/content/content-api.ts: the request goes through
 * the shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions — the server's honest
 * 503 for an unreachable PostgreSQL included), then the JSON payload is
 * validated into a DTO.
 */

const DEMO_PATH = '/demo/db/orm-probe';

export class ORMProbeApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/orm-probe — the measured exhibit. */
  async get(): Promise<ORMProbeDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, ormProbeDtoFromJson);
    });
  }
}
