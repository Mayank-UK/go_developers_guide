import type { Result } from '../../core/result';
import type { ScenarioRepository } from './scenario-repository';
import type { ScenarioState } from './scenario';

/**
 * GetScenarioUseCase — loads the demo scenario's current snapshot for the
 * header panel. A thin orchestration seam over the repository port
 * (react-guide use-case style); the port→adapter wiring happens at the
 * composition root.
 */
export class GetScenarioUseCase {
  private readonly repository: ScenarioRepository;

  constructor(repository: ScenarioRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ScenarioState>> {
    return this.repository.getScenario();
  }
}
