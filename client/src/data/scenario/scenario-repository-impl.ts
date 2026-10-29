import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ScenarioPatch, ScenarioState } from '../../domain/scenario/scenario';
import type { ScenarioRepository } from '../../domain/scenario/scenario-repository';
import { toScenarioState } from './scenario-dto';
import type { ScenarioApi } from './scenario-api';

/**
 * ScenarioRepositoryImpl — adapts the scenario API to the domain port. Every
 * thrown transport error (network, non-2xx envelope, malformed payload) is
 * guarded by safeRepositoryCall into a `failed` Result with its machine
 * diagnostics, so use cases and the embedded scenario panel never need try/catch and no
 * rejection goes unhandled.
 */
export class ScenarioRepositoryImpl implements ScenarioRepository {
  private readonly api: ScenarioApi;

  constructor(api: ScenarioApi) {
    this.api = api;
  }

  getScenario(): Promise<Result<ScenarioState>> {
    return safeRepositoryCall(async () => toScenarioState(await this.api.getScenario()));
  }

  patchScenario(patch: ScenarioPatch): Promise<Result<ScenarioState>> {
    return safeRepositoryCall(async () =>
      toScenarioState(await this.api.patchScenario(patch)),
    );
  }
}
