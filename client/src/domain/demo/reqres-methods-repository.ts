import type { Result } from '../../core/result';
import type { ReqresMethodsDemoData } from './reqres-methods';

/**
 * ReqresMethodsRepository — the domain-facing port for the methods
 * report demo payload. Demo components depend on this interface only;
 * the HTTP adapter lives in data/demo and is wired at the composition
 * root.
 */
export interface ReqresMethodsRepository {
  /** The multi-method exhibit's observed report, plus the SSE beat. */
  get(): Promise<Result<ReqresMethodsDemoData>>;
}
