import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import {
  concurrencyDtoFromJson,
  type ConcurrencyDemoDto,
} from './concurrency-dto';

/**
 * ConcurrencyApi — the transport for GET /api/demo/runtime/concurrency,
 * in the same style as data/demo/runtime-types-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO. Converting typed exceptions into
 * Failed Results is the repository adapter's job.
 */

const DEMO_PATH = '/demo/runtime/concurrency';

export class ConcurrencyApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/runtime/concurrency — the worker-pool transcript. */
  async get(): Promise<ConcurrencyDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, concurrencyDtoFromJson);
    });
  }
}
