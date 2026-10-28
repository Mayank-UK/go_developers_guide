import type { Result } from '../../core/result';
import type { ArchEventstoreRepository } from './arch-eventstore-repository';
import type { ArchEventstoreDemoData } from './arch-eventstore';

/**
 * ARCH_EVENTSTORE_DEMO_USE_CASE — the container token this demo's use
 * case is registered under (same aliasing story as every demo's
 * token); the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(ARCH_EVENTSTORE_DEMO_USE_CASE, new GetArchEventstoreDemoUseCase(repository));
 */
export const ARCH_EVENTSTORE_DEMO_USE_CASE = Symbol('GetArchEventstoreDemoUseCase');

/**
 * GetArchEventstoreDemoUseCase — loads the advanced-architecture
 * topic's event-store exhibit. Parameterless and repeatable: the
 * exhibit is a pure function of fixed inputs at a fixed exhibit clock,
 * so every execute serves a byte-identical payload — which is itself
 * part of the lesson (replay determinism, observed on every load).
 */
export class GetArchEventstoreDemoUseCase {
  private readonly repository: ArchEventstoreRepository;

  constructor(repository: ArchEventstoreRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ArchEventstoreDemoData>> {
    return this.repository.get();
  }
}
