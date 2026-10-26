import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { mongoConsoleDtoFromJson, type MongoConsoleDemoDto } from './mongo-console-dto';

/**
 * MongoConsoleApi — the transport for GET /api/demo/db/mongo-console,
 * in the same style as data/content/content-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions — the server's
 * honest 503 for an unreachable MongoDB included), then the JSON
 * payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/db/mongo-console';

export class MongoConsoleApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/mongo-console — the scripted console run
   * against the live MongoDB. */
  async get(): Promise<MongoConsoleDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, mongoConsoleDtoFromJson);
    });
  }
}
