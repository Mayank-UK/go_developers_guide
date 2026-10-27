import type { Result } from '../../core/result';
import type { CachingLabDemoData } from './caching-lab';

/**
 * CachingLabRepository — the domain-facing port for the caching topic's
 * demo payload. Demo components depend on this interface only; the HTTP
 * adapter lives in data/demo and is wired at the composition root.
 */
export interface CachingLabRepository {
  /** The lab's beats, herd, swap, and scoreboard. */
  get(): Promise<Result<CachingLabDemoData>>;
}
