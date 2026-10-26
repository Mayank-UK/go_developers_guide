import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { ContentIndex } from './content';
import type { ContentRepository } from './content-repository';
import { ListContentUseCase } from './list-content-usecase';

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(result: Result<ContentIndex>): ContentRepository {
  return {
    list: () => Promise.resolve(result),
    byId: () => {
      throw new Error('not expected in this use case');
    },
  };
}

const index: ContentIndex = {
  version: 1,
  groups: [{ id: 'foundations', title: 'Foundations', order: 1 }],
  topics: [],
};

describe('ListContentUseCase', () => {
  it('passes a successful repository result through as a Success', async () => {
    const useCase = new ListContentUseCase(fakeRepository(success(index)));

    await expect(useCase.execute()).resolves.toEqual(success(index));
  });

  it('passes a repository failure through as a Failed (never throws)', async () => {
    const failure = new NetworkFailure();
    const useCase = new ListContentUseCase(fakeRepository(failed(failure)));

    await expect(useCase.execute()).resolves.toEqual(failed(failure));
  });
});
