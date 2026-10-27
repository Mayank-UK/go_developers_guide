import type { Result } from '../../core/result';
import type { JobsConsoleRepository } from './jobs-console-repository';
import type { JobsConsoleDemoData } from './jobs-console';

/**
 * JOBS_CONSOLE_DEMO_USE_CASE — the container token this demo's use case
 * is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(JOBS_CONSOLE_DEMO_USE_CASE, new GetJobsConsoleDemoUseCase(repository));
 */
export const JOBS_CONSOLE_DEMO_USE_CASE = Symbol('GetJobsConsoleDemoUseCase');

/**
 * GetJobsConsoleDemoUseCase — loads the background-jobs topic's console
 * story payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the server mints the story's throwaway namespace and
 * walks the whole choreography fresh per request.
 */
export class GetJobsConsoleDemoUseCase {
  private readonly repository: JobsConsoleRepository;

  constructor(repository: JobsConsoleRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<JobsConsoleDemoData>> {
    return this.repository.get();
  }
}
