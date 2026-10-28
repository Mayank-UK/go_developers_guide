import type { Result } from '../../core/result';
import type { ReliabilityBreakerDemoData } from './reliability-breaker';

/**
 * ReliabilityBreakerRepository — the domain-facing port for the
 * reliability topic's breaker payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface ReliabilityBreakerRepository {
  /** The persistent breaker's state plus this request's walk. */
  get(): Promise<Result<ReliabilityBreakerDemoData>>;
}
