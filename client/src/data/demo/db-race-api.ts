import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { raceDtoFromJson, type RaceDemoDto } from './db-race-dto';

/** RaceApi — the transport for GET /api/demo/db/race. */

const DEMO_PATH = '/demo/db/race';

export class RaceApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/race — the choreographed isolation story. */
  async get(): Promise<RaceDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, raceDtoFromJson);
    });
  }
}
