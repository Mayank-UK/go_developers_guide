import type { Result } from '../../core/result';
import type { ArchSagaRepository } from './arch-saga-repository';
import type { ArchSagaDemoData } from './arch-saga';

/**
 * ARCH_SAGA_DEMO_USE_CASE — the container token this demo's use case is
 * registered under (same aliasing story as every demo's token); the
 * composition root registers the real pipeline under this exact symbol:
 *
 *   container.register(ARCH_SAGA_DEMO_USE_CASE, new GetArchSagaDemoUseCase(repository));
 */
export const ARCH_SAGA_DEMO_USE_CASE = Symbol('GetArchSagaDemoUseCase');

/**
 * GetArchSagaDemoUseCase — loads the advanced-architecture topic's
 * saga panel. Parameterless and repeatable: the runs are pure functions
 * of the fixed script, so each execute serves byte-identical ledgers —
 * the compensation records are the lesson, and replaying them is safe
 * by the machinery's own idempotency.
 */
export class GetArchSagaDemoUseCase {
  private readonly repository: ArchSagaRepository;

  constructor(repository: ArchSagaRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ArchSagaDemoData>> {
    return this.repository.get();
  }
}
