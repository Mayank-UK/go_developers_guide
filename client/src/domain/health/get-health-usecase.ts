import type { Result } from '../../core/result';
import type { HealthRepository } from './health-repository';
import type { HealthStatus } from './health-status';

/**
 * GetHealthUseCase — the liveness probe behind the home view's status
 * readout. A thin orchestration seam over the repository port (react-guide
 * use-case style); the port→adapter wiring happens at the composition root.
 */
export class GetHealthUseCase {
  private readonly repository: HealthRepository;

  constructor(repository: HealthRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<HealthStatus>> {
    return this.repository.getHealth();
  }
}
