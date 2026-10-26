import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { appInfoDtoFromJson, type AppInfoDemoDto } from './app-info-dto';

/**
 * AppInfoApi — the transport for GET /api/demo/gohttp/app-info, in the
 * same style as data/demo/runtime-types-api.ts: the request goes through
 * the shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON payload is
 * validated into a DTO. Converting typed exceptions into Failed Results is
 * the repository adapter's job.
 */

const DEMO_PATH = '/demo/gohttp/app-info';

export class AppInfoApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/gohttp/app-info — the running server's self-introduction. */
  async get(): Promise<AppInfoDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, appInfoDtoFromJson);
    });
  }
}
