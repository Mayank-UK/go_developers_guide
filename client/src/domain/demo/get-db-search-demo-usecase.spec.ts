import { describe, expect, it } from 'vitest';
import { failed, success } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import { GetSearchDemoUseCase } from './get-db-search-demo-usecase';
import type { SearchRepository } from './db-search-repository';
import type { SearchDemoData } from './db-search';

const fixture: SearchDemoData = {
  query: 'logging or errors',
  tsQuery: "'log' | 'error'",
  sql: 'SELECT …',
  matches: [],
  plan: [],
  indexDefinition: 'CREATE INDEX … gin …',
  teach: 'stem both sides',
};

describe('GetSearchDemoUseCase', () => {
  it('returns the repository payload on success', async () => {
    const useCase = new GetSearchDemoUseCase({
      get: () => Promise.resolve(success(fixture)),
    } as SearchRepository);
    const result = await useCase.execute();
    expect(result.ok && result.value.tsQuery).toBe("'log' | 'error'");
  });

  it('passes failures through untouched', async () => {
    const useCase = new GetSearchDemoUseCase({
      get: () => Promise.resolve(failed(new NetworkFailure('down'))),
    } as SearchRepository);
    expect((await useCase.execute()).ok).toBe(false);
  });
});
