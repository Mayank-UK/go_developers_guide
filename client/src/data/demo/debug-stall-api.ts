import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { debugStallDtoFromJson, type DebugStallDemoDto } from './debug-stall-dto';

/**
 * DebugStallApi — the transport for GET /api/demo/debug/stall, in the
 * same style as data/demo/testing-runner-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions), then the
 * JSON payload is validated into a DTO. A broken demo on the server
 * (a lost wedge) arrives as the typed 500 envelope and guards into a
 * ServerFailure here.
 */

const DEMO_PATH = '/demo/debug/stall';

export class DebugStallApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/debug/stall — one freshly staged stall. */
  async get(): Promise<DebugStallDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, debugStallDtoFromJson);
    });
  }
}
