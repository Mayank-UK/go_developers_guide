import type { Result } from '../../core/result';
import type { RedisPlaygroundRepository } from './redis-playground-repository';
import type { RedisPlaygroundDemoData } from './redis-playground';

/**
 * REDIS_PLAYGROUND_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(REDIS_PLAYGROUND_DEMO_USE_CASE, new GetRedisPlaygroundDemoUseCase(repository));
 */
export const REDIS_PLAYGROUND_DEMO_USE_CASE = Symbol('GetRedisPlaygroundDemoUseCase');

/**
 * GetRedisPlaygroundDemoUseCase — loads the redis topic's live-walk
 * payload. Thin orchestration seam over the repository port (react-guide
 * use-case style); parameterless because the demo has no user inputs —
 * the server mints the walk's throwaway namespace per request.
 */
export class GetRedisPlaygroundDemoUseCase {
  private readonly repository: RedisPlaygroundRepository;

  constructor(repository: RedisPlaygroundRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<RedisPlaygroundDemoData>> {
    return this.repository.get();
  }
}
