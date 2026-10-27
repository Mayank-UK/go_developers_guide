import type { Result } from '../../core/result';
import type { DebugStallRepository } from './debug-stall-repository';
import type { DebugStallDemoData } from './debug-stall';

/**
 * DEBUG_STALL_DEMO_USE_CASE — the container token this demo's use
 * case is registered under (same aliasing story as every demo's token);
 * the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(DEBUG_STALL_DEMO_USE_CASE, new GetDebugStallDemoUseCase(repository));
 */
export const DEBUG_STALL_DEMO_USE_CASE = Symbol('GetDebugStallDemoUseCase');

/**
 * GetDebugStallDemoUseCase — loads the debugging topic's stall
 * payload. Parameterless and repeatable: every execute asks the server
 * to stage a fresh wedge, capture it, and release it — the demo's
 * reload gesture is "capture again".
 */
export class GetDebugStallDemoUseCase {
  private readonly repository: DebugStallRepository;

  constructor(repository: DebugStallRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<DebugStallDemoData>> {
    return this.repository.get();
  }
}
