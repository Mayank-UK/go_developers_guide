import type { Result } from '../../core/result';
import type { ORMProbeDemoData } from './orm-probe';

/**
 * ORMProbeRepository — the domain-facing port for the sqlc-gorm topic's
 * measured data-access demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface ORMProbeRepository {
  /** The measured exhibit: dataset, four counted trials, unit of work. */
  get(): Promise<Result<ORMProbeDemoData>>;
}
