import type { Result } from '../../core/result';
import type { RuntimeTypesRepository } from './runtime-types-repository';
import type { RuntimeTypesDemoData } from './runtime-types';

/**
 * GetRuntimeTypesDemoUseCase — loads the go-language topic's runtime-types
 * demo payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * inputs — the server computes the proof from real guide objects.
 */
export class GetRuntimeTypesDemoUseCase {
  private readonly repository: RuntimeTypesRepository;

  constructor(repository: RuntimeTypesRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<RuntimeTypesDemoData>> {
    return this.repository.get();
  }
}
