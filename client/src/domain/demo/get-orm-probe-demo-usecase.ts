import type { Result } from '../../core/result';
import type { ORMProbeRepository } from './orm-probe-repository';
import type { ORMProbeDemoData } from './orm-probe';

/**
 * DB_ORM_PROBE_DEMO_USE_CASE — the container token this demo's use case
 * is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(DB_ORM_PROBE_DEMO_USE_CASE, new GetORMProbeDemoUseCase(repository));
 */
export const DB_ORM_PROBE_DEMO_USE_CASE = Symbol('GetORMProbeDemoUseCase');

/**
 * GetORMProbeDemoUseCase — loads the sqlc-gorm topic's measured
 * data-access exhibit. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the payload IS the measurement.
 */
export class GetORMProbeDemoUseCase {
  private readonly repository: ORMProbeRepository;

  constructor(repository: ORMProbeRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ORMProbeDemoData>> {
    return this.repository.get();
  }
}
