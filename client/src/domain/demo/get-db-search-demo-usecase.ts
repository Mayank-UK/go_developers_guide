import type { Result } from '../../core/result';
import type { SearchRepository } from './db-search-repository';
import type { SearchDemoData } from './db-search';

/** DB_SEARCH_DEMO_USE_CASE — the container token for this demo's use case. */
export const DB_SEARCH_DEMO_USE_CASE = Symbol('GetSearchDemoUseCase');

/** GetSearchDemoUseCase — loads the full-text search payload. */
export class GetSearchDemoUseCase {
  private readonly repository: SearchRepository;

  constructor(repository: SearchRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<SearchDemoData>> {
    return this.repository.get();
  }
}
