import type { Result } from '../../core/result';
import type { TestingRunnerDemoData } from './testing-runner';

/**
 * TestingRunnerRepository — the domain-facing port for the testing
 * topic's runner demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface TestingRunnerRepository {
  /** The boot-cached run's parsed timeline plus the counted shape. */
  get(): Promise<Result<TestingRunnerDemoData>>;
}
