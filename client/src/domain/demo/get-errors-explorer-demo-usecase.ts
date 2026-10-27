import type { Result } from '../../core/result';
import type { ErrorsExplorerRepository } from './errors-explorer-repository';
import type { ErrorsExplorerDemoData } from './errors-explorer';

/**
 * ERRORS_EXPLORER_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(ERRORS_EXPLORER_DEMO_USE_CASE, new GetErrorsExplorerDemoUseCase(repository));
 */
export const ERRORS_EXPLORER_DEMO_USE_CASE = Symbol('GetErrorsExplorerDemoUseCase');

/**
 * GetErrorsExplorerDemoUseCase — loads the error-handling topic's
 * explorer payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the server walks its probe under whatever failure
 * injection is armed at request time and derives the walk row from
 * what actually answered.
 */
export class GetErrorsExplorerDemoUseCase {
  private readonly repository: ErrorsExplorerRepository;

  constructor(repository: ErrorsExplorerRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ErrorsExplorerDemoData>> {
    return this.repository.get();
  }
}
