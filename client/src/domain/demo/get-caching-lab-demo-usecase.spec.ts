import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { CachingLabDemoData } from './caching-lab';
import type { CachingLabRepository } from './caching-lab-repository';
import { GetCachingLabDemoUseCase } from './get-caching-lab-demo-usecase';

/**
 * GetCachingLabDemoUseCase spec — the use case is a thin pass-through seam,
 * so the contract to pin is exactly that: whatever the repository resolves
 * (Success or Failed) arrives unchanged, and nothing throws.
 */

function fakeRepository(get: Result<CachingLabDemoData>): CachingLabRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: CachingLabDemoData = {
  ttlSeconds: 5,
  beats: [
    {
      id: 'cold-read',
      title: 'The first read misses',
      script: ['GetByID("p-1")'],
      outcome: 'miss',
      observedValue: 'Why Go for backends',
      detail: 'the store answered and the entry went in.',
      hits: 0,
      misses: 1,
      storeReads: 1,
    },
  ],
  herd: {
    id: 'p-2',
    readers: 12,
    storeReads: 1,
    sameAnswer: true,
    answer: 'ServeMux patterns in Go 1.22',
    proof: 'the herd coalesced into one store read.',
  },
  swap: {
    liveOnlyId: 'p-7',
    liveOnlyTitle: 'A seventh post, live only',
    notFoundId: 'p-7',
    notFoundSurfaced: true,
    fakeTitle: 'Why Go for backends (edited by the lab)',
    liveTitle: 'Why Go for backends',
    backTitle: 'Why Go for backends (edited by the lab)',
    proof: 'the partition salt kept the stores honest.',
  },
  stats: {
    hits: 13,
    misses: 10,
    expiries: 2,
    invalidations: 2,
    storeReads: 10,
    entries: 5,
    proof: 'misses equal store reads.',
  },
  reasoning: 'the response itself carries an ETag.',
};

describe('GetCachingLabDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetCachingLabDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetCachingLabDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
