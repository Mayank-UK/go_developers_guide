import type { Result } from '../../core/result';
import type { AuthzMatrixRepository } from './authz-matrix-repository';
import type { AuthzMatrixDemoData } from './authz-matrix';

/**
 * The container token for the authorization matrix demo's use case. It
 * lives here (not in container.ts's ContainerTokens) so the demo component
 * has its resolution seam from day one; the composition root registers a
 * GetAuthzMatrixDemoUseCase under it (ContainerTokens aliases it for
 * discovery).
 */
export const AUTHZ_MATRIX_DEMO_USE_CASE = Symbol('GetAuthzMatrixDemoUseCase');

/**
 * GetAuthzMatrixDemoUseCase — loads the authorization matrix payload.
 * Parameterless because the demo has no user inputs: the server evaluates
 * every subject × action × object cell fresh per request through its real
 * policy evaluator, so the payload is always the live verdict.
 */
export class GetAuthzMatrixDemoUseCase {
  private readonly repository: AuthzMatrixRepository;

  constructor(repository: AuthzMatrixRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<AuthzMatrixDemoData>> {
    return this.repository.get();
  }
}
