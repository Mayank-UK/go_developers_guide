import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ReliabilityBreakerRepository } from '../../domain/demo/reliability-breaker-repository';
import type { ReliabilityBreakerDemoData } from '../../domain/demo/reliability-breaker';
import { toReliabilityBreakerDemo } from './reliability-breaker-dto';
import type { ReliabilityBreakerApi } from './reliability-breaker-api';

/**
 * ReliabilityBreakerRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so
 * the demo component never needs try/catch and no rejection goes
 * unhandled.
 */
export class ReliabilityBreakerRepositoryImpl implements ReliabilityBreakerRepository {
  private readonly api: ReliabilityBreakerApi;

  constructor(api: ReliabilityBreakerApi) {
    this.api = api;
  }

  async get(): Promise<Result<ReliabilityBreakerDemoData>> {
    return safeRepositoryCall(async () => toReliabilityBreakerDemo(await this.api.get()));
  }
}
