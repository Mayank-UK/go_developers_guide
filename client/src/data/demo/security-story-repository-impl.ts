import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { SecurityStoryRepository } from '../../domain/demo/security-story-repository';
import type { SecurityStoryDemoData } from '../../domain/demo/security-story';
import { toSecurityStoryDemo } from './security-story-dto';
import type { SecurityStoryApi } from './security-story-api';

/**
 * SecurityStoryRepositoryImpl — adapts one story API instance to the
 * domain port. Every thrown transport error (network, non-2xx,
 * malformed payload) is guarded by safeRepositoryCall into a Failed
 * Result, so the demo components never need try/catch and no rejection
 * goes unhandled.
 */
export class SecurityStoryRepositoryImpl implements SecurityStoryRepository {
  private readonly api: SecurityStoryApi;

  constructor(api: SecurityStoryApi) {
    this.api = api;
  }

  async get(): Promise<Result<SecurityStoryDemoData>> {
    return safeRepositoryCall(async () => toSecurityStoryDemo(await this.api.get()));
  }
}
