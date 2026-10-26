import type { Result } from '../../core/result';
import type { ScenarioRepository } from './scenario-repository';
import type { ScenarioPatch, ScenarioState } from './scenario';

/**
 * PatchScenarioUseCase — applies a partial scenario update (the header panel's
 * select changes) and answers with the resulting full state. Thin
 * orchestration seam over the repository port (react-guide use-case style);
 * the port→adapter wiring happens at the composition root.
 */
export class PatchScenarioUseCase {
  private readonly repository: ScenarioRepository;

  constructor(repository: ScenarioRepository) {
    this.repository = repository;
  }

  execute(patch: ScenarioPatch): Promise<Result<ScenarioState>> {
    return this.repository.patchScenario(patch);
  }
}
