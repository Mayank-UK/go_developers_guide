import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { SearchRepository } from '../../domain/demo/db-search-repository';
import type { SearchDemoData } from '../../domain/demo/db-search';
import { toSearchDemo } from './db-search-dto';
import type { SearchApi } from './db-search-api';

/** SearchRepositoryImpl — adapts the demo API to the domain port. */
export class SearchRepositoryImpl implements SearchRepository {
  private readonly api: SearchApi;

  constructor(api: SearchApi) {
    this.api = api;
  }

  async get(): Promise<Result<SearchDemoData>> {
    return safeRepositoryCall(async () => toSearchDemo(await this.api.get()));
  }
}
