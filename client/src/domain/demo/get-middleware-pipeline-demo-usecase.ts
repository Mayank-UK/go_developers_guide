import type { Result } from '../../core/result';
import type { MiddlewarePipelineRepository } from './middleware-pipeline-repository';
import type { MiddlewarePipelineDemoData } from './middleware-pipeline';

/**
 * MIDDLEWARE_PIPELINE_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(MIDDLEWARE_PIPELINE_DEMO_USE_CASE, new GetMiddlewarePipelineDemoUseCase(repository));
 */
export const MIDDLEWARE_PIPELINE_DEMO_USE_CASE = Symbol('GetMiddlewarePipelineDemoUseCase');

/**
 * GetMiddlewarePipelineDemoUseCase — loads the middleware topic's
 * pipeline demo payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the payload describes the run that fetched it.
 */
export class GetMiddlewarePipelineDemoUseCase {
  private readonly repository: MiddlewarePipelineRepository;

  constructor(repository: MiddlewarePipelineRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<MiddlewarePipelineDemoData>> {
    return this.repository.get();
  }
}
