import type { Result } from '../../core/result';
import type { EnvInspectorRepository } from './env-inspector-repository';
import type { EnvInspectorDemoData } from './env-inspector';

/**
 * ENV_INSPECTOR_DEMO_USE_CASE — the container token this demo's use
 * case is registered under (same aliasing story as every demo's token);
 * the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(ENV_INSPECTOR_DEMO_USE_CASE, new GetEnvInspectorDemoUseCase(repository));
 */
export const ENV_INSPECTOR_DEMO_USE_CASE = Symbol('GetEnvInspectorDemoUseCase');

/**
 * GetEnvInspectorDemoUseCase — loads the environments topic's env
 * inspector payload. Parameterless and repeatable: every execute asks
 * the server to derive the payload fresh — attribution re-reads the
 * environment and the flag panel snapshots the ScenarioStore at request
 * time, so the demo's reload gesture is "derive again".
 */
export class GetEnvInspectorDemoUseCase {
  private readonly repository: EnvInspectorRepository;

  constructor(repository: EnvInspectorRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<EnvInspectorDemoData>> {
    return this.repository.get();
  }
}
