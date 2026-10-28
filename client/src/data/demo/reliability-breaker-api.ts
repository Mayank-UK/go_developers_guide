import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { reliabilityBreakerDtoFromJson, type ReliabilityBreakerDemoDto } from './reliability-breaker-dto';

/**
 * ReliabilityBreakerApi — the transport for
 * GET /api/demo/reliability/breaker, in the same style as
 * data/demo/perf-bench-api.ts: the request goes through the shared
 * guarded fetch pipeline (network down / non-JSON / non-2xx error
 * envelope all become typed AppExceptions), then the JSON payload is
 * validated into a DTO.
 */

const DEMO_PATH = '/demo/reliability/breaker';

export class ReliabilityBreakerApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/reliability/breaker — the breaker's live state. */
  async get(): Promise<ReliabilityBreakerDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, reliabilityBreakerDtoFromJson);
    });
  }
}
