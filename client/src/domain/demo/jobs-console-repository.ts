import type { Result } from '../../core/result';
import type { JobsConsoleDemoData } from './jobs-console';

/**
 * JobsConsoleRepository — the domain-facing port for the
 * background-jobs topic's console-story demo payload. Demo components
 * depend on this interface only; the HTTP adapter lives in data/demo
 * and is wired at the composition root.
 */
export interface JobsConsoleRepository {
  /** The story's queues, ladders, beats, and proofs. */
  get(): Promise<Result<JobsConsoleDemoData>>;
}
