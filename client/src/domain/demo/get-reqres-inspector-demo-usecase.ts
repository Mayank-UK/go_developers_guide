import type { Result } from '../../core/result';
import type { ReqresInspectorRepository } from './reqres-inspector-repository';
import type { ReqresInspectorDemoData } from './reqres-inspector';

/**
 * REQRES_INSPECTOR_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(REQRES_INSPECTOR_DEMO_USE_CASE, new GetReqresInspectorDemoUseCase(repository));
 */
export const REQRES_INSPECTOR_DEMO_USE_CASE = Symbol('GetReqresInspectorDemoUseCase');

/**
 * GetReqresInspectorDemoUseCase — loads the request-response topic's
 * inspector demo payload. Thin orchestration seam over the repository
 * port (react-guide use-case style); parameterless because the demo has
 * no user inputs — the payload describes the fetch that fetched it.
 */
export class GetReqresInspectorDemoUseCase {
  private readonly repository: ReqresInspectorRepository;

  constructor(repository: ReqresInspectorRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ReqresInspectorDemoData>> {
    return this.repository.get();
  }
}
