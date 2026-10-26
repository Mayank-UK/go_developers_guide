import type { Result } from '../../core/result';
import type { InterfaceMatrixDemoData } from './interface-matrix';

/**
 * InterfaceMatrixRepository — the domain-facing port for the
 * satisfaction-matrix demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface InterfaceMatrixRepository {
  /** The full matrix payload, every verdict computed live by the server. */
  get(): Promise<Result<InterfaceMatrixDemoData>>;
}
