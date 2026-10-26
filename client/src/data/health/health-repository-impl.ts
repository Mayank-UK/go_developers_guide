import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { HealthRepository } from '../../domain/health/health-repository';
import type { HealthStatus } from '../../domain/health/health-status';
import type { HealthApi } from './health-api';

/**
 * HealthRepositoryImpl — adapts the health API to the domain port. Every
 * thrown transport error (network, non-2xx, malformed payload) is guarded by
 * safeRepositoryCall into a `failed` Result with its machine diagnostics, so
 * use cases and views never need try/catch and no rejection goes unhandled.
 */
export class HealthRepositoryImpl implements HealthRepository {
  private readonly api: HealthApi;

  constructor(api: HealthApi) {
    this.api = api;
  }

  async getHealth(): Promise<Result<HealthStatus>> {
    return safeRepositoryCall(async () => {
      const payload = await this.api.getHealth();
      return { status: payload.status, env: payload.env };
    });
  }
}
