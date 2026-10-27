import type { Result } from '../../core/result';
import type { DebugStallDemoData } from './debug-stall';

/**
 * DebugStallRepository — the domain-facing port for the debugging
 * topic's stall demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface DebugStallRepository {
  /** One freshly staged stall: wedge, capture, parse, release. */
  get(): Promise<Result<DebugStallDemoData>>;
}
