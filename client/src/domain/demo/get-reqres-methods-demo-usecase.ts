import type { Result } from '../../core/result';
import type { ReqresMethodsRepository } from './reqres-methods-repository';
import type { ReqresMethodsDemoData } from './reqres-methods';

/**
 * REQRES_METHODS_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(REQRES_METHODS_DEMO_USE_CASE, new GetReqresMethodsDemoUseCase(repository));
 */
export const REQRES_METHODS_DEMO_USE_CASE = Symbol('GetReqresMethodsDemoUseCase');

/**
 * GetReqresMethodsDemoUseCase — loads the request-response topic's
 * methods report payload. Thin orchestration seam over the repository
 * port (react-guide use-case style); parameterless because the demo has
 * no user inputs — the exhibit runs in-process and the payload is
 * byte-for-byte deterministic.
 */
export class GetReqresMethodsDemoUseCase {
  private readonly repository: ReqresMethodsRepository;

  constructor(repository: ReqresMethodsRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ReqresMethodsDemoData>> {
    return this.repository.get();
  }
}
