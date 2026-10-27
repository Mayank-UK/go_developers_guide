import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { AuthStoryRepository } from '../../domain/demo/auth-story-repository';
import type { AuthStoryDemoData } from '../../domain/demo/auth-story';
import { toAuthStoryDemo } from './auth-story-dto';
import type { AuthStoryApi } from './auth-story-api';

/**
 * AuthStoryRepositoryImpl — adapts one story API instance to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so
 * the demo components never need try/catch and no rejection goes
 * unhandled.
 */
export class AuthStoryRepositoryImpl implements AuthStoryRepository {
  private readonly api: AuthStoryApi;

  constructor(api: AuthStoryApi) {
    this.api = api;
  }

  async get(): Promise<Result<AuthStoryDemoData>> {
    return safeRepositoryCall(async () => toAuthStoryDemo(await this.api.get()));
  }
}
