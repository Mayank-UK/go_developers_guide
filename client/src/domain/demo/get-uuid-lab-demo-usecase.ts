import type { Result } from '../../core/result';
import type { UUIDLabRepository } from './uuid-lab-repository';
import type { UUIDLabDemoData } from './uuid-lab';

/**
 * DB_UUID_DEMO_USE_CASE — the container token this demo's use case is
 * registered under. Lives here (not in container.ts's ContainerTokens) so
 * the demo component has its resolution seam from day one; the
 * composition root registers the real pipeline under this exact symbol:
 *
 *   container.register(DB_UUID_DEMO_USE_CASE, new GetUUIDLabDemoUseCase(repository));
 */
export const DB_UUID_DEMO_USE_CASE = Symbol('GetUUIDLabDemoUseCase');

/**
 * GetUUIDLabDemoUseCase — loads the sql-schema topic's UUIDv7 lab
 * payload. Thin orchestration seam over the repository port; every load
 * is a FRESH batch (the server mints on request), which is the demo's
 * point — the properties hold for every batch.
 */
export class GetUUIDLabDemoUseCase {
  private readonly repository: UUIDLabRepository;

  constructor(repository: UUIDLabRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<UUIDLabDemoData>> {
    return this.repository.get();
  }
}
