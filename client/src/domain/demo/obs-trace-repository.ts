import type { Result } from '../../core/result';
import type { ObsTraceDemoData } from './obs-trace';

/**
 * ObsTraceRepository — the domain-facing port for the observability
 * topic's trace round-trip demo payload. Demo components depend on
 * this interface only; the HTTP adapter lives in data/demo and is
 * wired at the composition root.
 */
export interface ObsTraceRepository {
  /** The acceptance panel, the span tree, and the malformed table. */
  get(): Promise<Result<ObsTraceDemoData>>;
}
