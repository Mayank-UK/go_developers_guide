import type { Result } from '../../core/result';
import type { ConcurrencyRepository } from './concurrency-repository';
import type { ConcurrencyDemoData } from './concurrency';

/**
 * GetConcurrencyDemoUseCase — loads the go-runtime topic's concurrency
 * demo payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * inputs — the server runs the whole choreography per request.
 */
export class GetConcurrencyDemoUseCase {
  private readonly repository: ConcurrencyRepository;

  constructor(repository: ConcurrencyRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ConcurrencyDemoData>> {
    return this.repository.get();
  }
}
