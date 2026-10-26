import type { Result } from '../../core/result';
import type { MongoConsoleRepository } from './mongo-console-repository';
import type { MongoConsoleDemoData } from './mongo-console';

/**
 * MONGO_CONSOLE_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(MONGO_CONSOLE_DEMO_USE_CASE, new GetMongoConsoleDemoUseCase(repository));
 */
export const MONGO_CONSOLE_DEMO_USE_CASE = Symbol('GetMongoConsoleDemoUseCase');

/**
 * GetMongoConsoleDemoUseCase — loads the mongodb topic's console
 * payload. Thin orchestration seam over the repository port (react-guide
 * use-case style); parameterless because the demo has no user inputs —
 * the payload replays the server's scripted run, deterministically.
 */
export class GetMongoConsoleDemoUseCase {
  private readonly repository: MongoConsoleRepository;

  constructor(repository: MongoConsoleRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<MongoConsoleDemoData>> {
    return this.repository.get();
  }
}
