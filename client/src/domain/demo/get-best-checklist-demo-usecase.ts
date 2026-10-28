import type { Result } from '../../core/result';
import type { BestChecklistRepository } from './best-checklist-repository';
import type { BestChecklistDemoData } from './best-checklist';

/**
 * BEST_CHECKLIST_DEMO_USE_CASE — the container token this demo's use
 * case is registered under (same aliasing story as every demo's
 * token); the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(BEST_CHECKLIST_DEMO_USE_CASE, new GetBestChecklistDemoUseCase(repository));
 */
export const BEST_CHECKLIST_DEMO_USE_CASE = Symbol('GetBestChecklistDemoUseCase');

/**
 * GetBestChecklistDemoUseCase — loads the best-practices topic's
 * self-audit panel. Parameterless and repeatable: each execute serves
 * the SAME server-side boot snapshot, so the demo's reload gesture
 * replays the identical verdicts — which is itself part of the lesson.
 */
export class GetBestChecklistDemoUseCase {
  private readonly repository: BestChecklistRepository;

  constructor(repository: BestChecklistRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<BestChecklistDemoData>> {
    return this.repository.get();
  }
}
