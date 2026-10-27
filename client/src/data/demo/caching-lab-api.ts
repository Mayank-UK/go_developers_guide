import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { cachingLabDtoFromJson, type CachingLabDemoDto } from './caching-lab-dto';

/**
 * CachingLabApi — the transport for GET /api/demo/caching/lab, in the same
 * style as data/content/content-api.ts: the request goes through the shared
 * guarded fetch pipeline (network down / non-JSON / non-2xx error envelope
 * all become typed AppExceptions), then the JSON payload is validated into
 * a DTO. The endpoint's ETag/304 story is HTTP-level — the browser's fetch
 * handles validators transparently, so the pipeline just sees the 200 (or
 * the honest error envelope).
 */

const DEMO_PATH = '/demo/caching/lab';

export class CachingLabApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/caching/lab — the lab's beats and censuses. */
  async get(): Promise<CachingLabDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, cachingLabDtoFromJson);
    });
  }
}
