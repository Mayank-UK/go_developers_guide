import type { Result } from '../../core/result';
import type { ErrorsExplorerDemoData } from './errors-explorer';

/**
 * ErrorsExplorerRepository — the domain-facing port for the
 * error-handling topic's explorer demo payload. Demo components depend
 * on this interface only; the HTTP adapter lives in data/demo and is
 * wired at the composition root.
 */
export interface ErrorsExplorerRepository {
  /** The taxonomy table, the scenario/probe panels, and the live walk. */
  get(): Promise<Result<ErrorsExplorerDemoData>>;
}
