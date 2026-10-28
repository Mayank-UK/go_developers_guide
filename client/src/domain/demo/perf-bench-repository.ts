import type { Result } from '../../core/result';
import type { PerfBenchDemoData } from './perf-bench';

/**
 * PerfBenchRepository — the domain-facing port for the performance
 * topic's bench payload. Demo components depend on this interface
 * only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface PerfBenchRepository {
  /** The boot-cached benchmark run, parsed with its configuration. */
  get(): Promise<Result<PerfBenchDemoData>>;
}
