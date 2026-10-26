import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { redisPlaygroundDtoFromJson, type RedisPlaygroundDemoDto } from './redis-playground-dto';

/**
 * RedisPlaygroundApi — the transport for GET /api/demo/redis/playground,
 * in the same style as data/content/content-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions — the server's
 * honest 503 for an unreachable Redis included), then the JSON payload
 * is validated into a DTO.
 */

const DEMO_PATH = '/demo/redis/playground';

export class RedisPlaygroundApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/redis/playground — the live walk's rows. */
  async get(): Promise<RedisPlaygroundDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, redisPlaygroundDtoFromJson);
    });
  }
}
