import type { Result } from '../../core/result';
import type { ConcurrencyDemoData } from './concurrency';

/**
 * ConcurrencyRepository — the domain-facing port for the concurrency demo
 * payload. Demo components depend on this interface only; the HTTP adapter
 * lives in data/demo and is wired at the composition root.
 */
export interface ConcurrencyRepository {
  /** The worker-pool transcript, computed live by the server on every call. */
  get(): Promise<Result<ConcurrencyDemoData>>;
}
