import { describe, expect, it } from 'vitest';
import { SearchRepositoryImpl } from './db-search-repository-impl';
import type { SearchApi } from './db-search-api';
import type { SearchDemoData } from '../../domain/demo/db-search';

const fixture = {
  query: 'q',
  tsQuery: 't',
  sql: 's',
  matches: [],
  plan: [],
  indexDefinition: 'i',
  teach: 't',
} as SearchDemoData;

describe('SearchRepositoryImpl', () => {
  it('wraps the api payload in a success Result', async () => {
    const repo = new SearchRepositoryImpl({ get: async () => fixture } as unknown as SearchApi);
    expect((await repo.get()).ok).toBe(true);
  });

  it('guards api rejections into failed Results', async () => {
    const repo = new SearchRepositoryImpl({
      get: async () => { throw new Error('boom'); },
    } as unknown as SearchApi);
    expect((await repo.get()).ok).toBe(false);
  });
});
