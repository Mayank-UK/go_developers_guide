import type { Result } from '../../core/result';
import type { ReliabilityBreakerRepository } from './reliability-breaker-repository';
import type { ReliabilityBreakerDemoData } from './reliability-breaker';

/**
 * RELIABILITY_BREAKER_DEMO_USE_CASE — the container token this demo's
 * use case is registered under (same aliasing story as every demo's
 * token); the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(RELIABILITY_BREAKER_DEMO_USE_CASE, new GetReliabilityBreakerDemoUseCase(repository));
 */
export const RELIABILITY_BREAKER_DEMO_USE_CASE = Symbol('GetReliabilityBreakerDemoUseCase');

/**
 * GetReliabilityBreakerDemoUseCase — loads the reliability topic's
 * breaker payload. Parameterless and repeatable: each execute drives a
 * fresh walk through the SAME persistent server-side breaker, so the
 * demo's reload gesture is "walk it again" — the state the next walk
 * meets is the state the last walk left behind.
 */
export class GetReliabilityBreakerDemoUseCase {
  private readonly repository: ReliabilityBreakerRepository;

  constructor(repository: ReliabilityBreakerRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ReliabilityBreakerDemoData>> {
    return this.repository.get();
  }
}
