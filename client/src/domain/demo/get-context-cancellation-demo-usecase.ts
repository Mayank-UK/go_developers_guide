import type { Result } from '../../core/result';
import type { ContextCancellationRepository } from './context-cancellation-repository';
import type { ContextCancellationDemoData } from './context-cancellation';

/**
 * GetContextCancellationDemoUseCase — loads the goroutines-and-channels
 * topic's context-cancellation demo payload. Thin orchestration seam over
 * the repository port (react-guide use-case style); parameterless because
 * the demo has no inputs — the server performs the choreography from its
 * own configuration.
 */
export class GetContextCancellationDemoUseCase {
  private readonly repository: ContextCancellationRepository;

  constructor(repository: ContextCancellationRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ContextCancellationDemoData>> {
    return this.repository.get();
  }
}
