import type { Result } from '../../core/result';
import type { SearchDemoData } from './db-search';

/** SearchRepository — the domain-facing port for the search demo payload. */
export interface SearchRepository {
  /** The ranked tsvector match with its plan and index. */
  get(): Promise<Result<SearchDemoData>>;
}
