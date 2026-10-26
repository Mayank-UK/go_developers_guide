import type { Result } from '../../core/result';
import type { ValidationArenaRepository } from './validation-arena-repository';
import type { ValidationArenaDemoData } from './validation-arena';

/**
 * VALIDATION_ARENA_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(VALIDATION_ARENA_DEMO_USE_CASE, new GetValidationArenaDemoUseCase(repository));
 */
export const VALIDATION_ARENA_DEMO_USE_CASE = Symbol('GetValidationArenaDemoUseCase');

/**
 * GetValidationArenaDemoUseCase — loads the validation topic's arena demo
 * payload. Thin orchestration seam over the repository port (react-guide
 * use-case style); parameterless because the arena's candidate set is
 * builder-chosen server-side — the demo has no user inputs.
 */
export class GetValidationArenaDemoUseCase {
  private readonly repository: ValidationArenaRepository;

  constructor(repository: ValidationArenaRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ValidationArenaDemoData>> {
    return this.repository.get();
  }
}
