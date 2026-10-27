import type { Result } from '../../core/result';
import type { AuthzMatrixDemoData } from './authz-matrix';

/**
 * AuthzMatrixRepository — the domain-facing port for the authorization
 * topic's matrix demo. The demo component depends on this interface only;
 * the HTTP adapter lives in data/demo and is wired at the composition
 * root.
 */
export interface AuthzMatrixRepository {
  /** The matrix story's beats, proofs, and reasoning. */
  get(): Promise<Result<AuthzMatrixDemoData>>;
}
