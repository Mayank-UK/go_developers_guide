import type { Result } from '../../core/result';
import type { CachingLabRepository } from './caching-lab-repository';
import type { CachingLabDemoData } from './caching-lab';

/**
 * CACHING_LAB_DEMO_USE_CASE — the container token this demo's use case is
 * registered under. Lives here (not in container.ts's ContainerTokens) so
 * the demo component has its resolution seam from day one; the composition
 * root registers the real pipeline under this exact symbol:
 *
 *   container.register(CACHING_LAB_DEMO_USE_CASE, new GetCachingLabDemoUseCase(repository));
 */
export const CACHING_LAB_DEMO_USE_CASE = Symbol('GetCachingLabDemoUseCase');

/**
 * GetCachingLabDemoUseCase — loads the caching topic's lab payload. Thin
 * orchestration seam over the repository port (react-guide use-case style);
 * parameterless because the demo has no user inputs — the server runs the
 * whole choreography per request.
 */
export class GetCachingLabDemoUseCase {
  private readonly repository: CachingLabRepository;

  constructor(repository: CachingLabRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<CachingLabDemoData>> {
    return this.repository.get();
  }
}
