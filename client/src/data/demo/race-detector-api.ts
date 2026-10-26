import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { raceDetectorDtoFromJson, type RaceDetectorDemoDto } from './race-detector-dto';

/**
 * RaceDetectorApi — the transport for GET /api/demo/conc/race, in the
 * same style as data/demo/runtime-types-api.ts: the request goes through
 * the shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON payload
 * is validated into a DTO. Converting typed exceptions into Failed
 * Results is the repository adapter's job.
 */

const DEMO_PATH = '/demo/conc/race';

export class RaceDetectorApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/conc/race — the pinned verdict + live totals. */
  async get(): Promise<RaceDetectorDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, raceDetectorDtoFromJson);
    });
  }
}
