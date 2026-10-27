import type { Result } from '../../core/result';
import type { ObsViewerDemoData } from './obs-viewer';

/**
 * ObsViewerRepository — the domain-facing port for the observability
 * topic's slog viewer demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface ObsViewerRepository {
  /** The dissected line, the level matrix, and the correlated replay. */
  get(): Promise<Result<ObsViewerDemoData>>;
}
