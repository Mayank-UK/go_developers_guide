import type { Result } from '../../core/result';
import type { SQLSafetyDemoData } from './sql-safety';

/**
 * SQLSafetyRepository — the domain-facing port for the sql-safety demo
 * payload. Demo components depend on this interface only; the HTTP
 * adapter lives in data/demo and is wired at the composition root.
 */
export interface SQLSafetyRepository {
  /** The parameterized-vs-concatenated exhibit, run live. */
  get(): Promise<Result<SQLSafetyDemoData>>;
}
