import type { Result } from '../../core/result';
import type { ScalingStatelessRepository } from './scaling-stateless-repository';
import type { ScalingStatelessDemoData } from './scaling-stateless';

/**
 * SCALING_STATELESS_DEMO_USE_CASE — the container token this demo's
 * use case is registered under (same aliasing story as every demo's
 * token); the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(SCALING_STATELESS_DEMO_USE_CASE, new GetScalingStatelessDemoUseCase(repository));
 */
export const SCALING_STATELESS_DEMO_USE_CASE = Symbol('GetScalingStatelessDemoUseCase');

/**
 * GetScalingStatelessDemoUseCase — loads the scaling topic's
 * statelessness round. Parameterless and repeatable: each execute
 * drives a FRESH server-side fleet through the identical deterministic
 * choreography, so the demo's reload gesture replays the same measured
 * round — which is itself part of the lesson.
 */
export class GetScalingStatelessDemoUseCase {
  private readonly repository: ScalingStatelessRepository;

  constructor(repository: ScalingStatelessRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ScalingStatelessDemoData>> {
    return this.repository.get();
  }
}
