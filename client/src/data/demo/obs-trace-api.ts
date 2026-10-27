import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { obsTraceDtoFromJson, type ObsTraceDemoDto } from './obs-trace-dto';

/**
 * ObsTraceApi — the transport for GET /api/demo/obs/trace, in the
 * same style as data/demo/arch-tracer-api.ts: the request goes through
 * the shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/obs/trace';

export class ObsTraceApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/obs/trace — the round-trip + span tree. */
  async get(): Promise<ObsTraceDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, obsTraceDtoFromJson);
    });
  }
}
