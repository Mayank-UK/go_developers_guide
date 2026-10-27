import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { CachingLabRepository } from '../../domain/demo/caching-lab-repository';
import type { CachingLabDemoData } from '../../domain/demo/caching-lab';
import { toCachingLabDemo } from './caching-lab-dto';
import type { CachingLabApi } from './caching-lab-api';

/**
 * CachingLabRepositoryImpl — adapts the demo API to the domain port. Every
 * thrown transport error (network, non-2xx, malformed payload) is guarded
 * by safeRepositoryCall into a Failed Result, so the demo component never
 * needs try/catch and no rejection goes unhandled.
 */
export class CachingLabRepositoryImpl implements CachingLabRepository {
  private readonly api: CachingLabApi;

  constructor(api: CachingLabApi) {
    this.api = api;
  }

  async get(): Promise<Result<CachingLabDemoData>> {
    return safeRepositoryCall(async () => toCachingLabDemo(await this.api.get()));
  }
}
