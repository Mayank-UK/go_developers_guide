import type { Result } from '../../core/result';
import type { InterfaceMatrixRepository } from './interface-matrix-repository';
import type { InterfaceMatrixDemoData } from './interface-matrix';

/**
 * INTERFACE_MATRIX_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(INTERFACE_MATRIX_DEMO_USE_CASE, new GetInterfaceMatrixDemoUseCase(repository));
 */
export const INTERFACE_MATRIX_DEMO_USE_CASE = Symbol('GetInterfaceMatrixDemoUseCase');

/**
 * GetInterfaceMatrixDemoUseCase — loads the interfaces-and-composition
 * topic's satisfaction-matrix demo payload. Thin orchestration seam over
 * the repository port (react-guide use-case style); parameterless because
 * the demo has no inputs — the server computes every verdict from real
 * method sets.
 */
export class GetInterfaceMatrixDemoUseCase {
  private readonly repository: InterfaceMatrixRepository;

  constructor(repository: InterfaceMatrixRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<InterfaceMatrixDemoData>> {
    return this.repository.get();
  }
}
