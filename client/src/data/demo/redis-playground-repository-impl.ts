import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { RedisPlaygroundRepository } from '../../domain/demo/redis-playground-repository';
import type { RedisPlaygroundDemoData } from '../../domain/demo/redis-playground';
import { toRedisPlaygroundDemo } from './redis-playground-dto';
import type { RedisPlaygroundApi } from './redis-playground-api';

/**
 * RedisPlaygroundRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class RedisPlaygroundRepositoryImpl implements RedisPlaygroundRepository {
  private readonly api: RedisPlaygroundApi;

  constructor(api: RedisPlaygroundApi) {
    this.api = api;
  }

  async get(): Promise<Result<RedisPlaygroundDemoData>> {
    return safeRepositoryCall(async () => toRedisPlaygroundDemo(await this.api.get()));
  }
}
