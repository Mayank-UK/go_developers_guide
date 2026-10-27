import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { CachingLabApi } from './caching-lab-api';
import { CachingLabRepositoryImpl } from './caching-lab-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * CachingLabRepositoryImpl spec — fixture-driven (happy + failure): the lab
 * payload (beats, herd, swap, scoreboard) round-trips; a poisoned payload
 * and a network failure resolve as Failed Results, never throws.
 */

const demoPayload = {
  ttlSeconds: 5,
  beats: [
    {
      id: 'cold-read',
      title: 'The first read misses',
      script: ['GetByID("p-1")'],
      outcome: 'miss',
      observedValue: 'Why Go for backends',
      detail: 'The cache was cold: the store answered and the entry went in (1 store reads so far).',
      hits: 0,
      misses: 1,
      storeReads: 1,
    },
    {
      id: 'warm-read',
      title: 'The second read hits',
      script: ['GetByID("p-1")'],
      outcome: 'hit',
      observedValue: 'Why Go for backends',
      detail: 'Served "Why Go for backends" from memory — the store-read counter did not move (1 total).',
      hits: 1,
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
    proof: '12 concurrent readers of p-2 made the store answer exactly 1 read.',
  },
  swap: {
    liveOnlyId: 'p-7',
    liveOnlyTitle: 'A seventh post, live only',
    notFoundId: 'p-7',
    notFoundSurfaced: true,
    fakeTitle: 'Why Go for backends (edited by the lab)',
    liveTitle: 'Why Go for backends',
    backTitle: 'Why Go for backends (edited by the lab)',
    proof: 'one store\u2019s rows never answer for the other.',
  },
  stats: {
    hits: 13,
    misses: 10,
    expiries: 2,
    invalidations: 2,
    storeReads: 10,
    entries: 5,
    proof: '10 misses and 10 store reads — equal by construction.',
  },
  reasoning: 'the response itself carries an ETag.',
};

function makeRepository(): CachingLabRepositoryImpl {
  return new CachingLabRepositoryImpl(new CachingLabApi(createAppConfig()));
}

describe('CachingLabRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/caching/lab', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.ttlSeconds).toBe(5);
      expect(result.value.beats).toHaveLength(2);
      expect(result.value.beats[0]?.outcome).toBe('miss');
      expect(result.value.beats[1]?.outcome).toBe('hit');
      expect(result.value.herd.readers).toBe(12);
      expect(result.value.herd.storeReads).toBe(1);
      expect(result.value.swap.notFoundSurfaced).toBe(true);
      expect(result.value.stats.misses).toBe(result.value.stats.storeReads);
    }
  });

  it('rejects a payload whose beat outcome is unknown', async () => {
    const poisoned = {
      ...demoPayload,
      beats: [{ ...demoPayload.beats[0], outcome: 'cached-ish' }],
    };
    stubApiFetch([{ url: '/api/demo/caching/lab', body: poisoned }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
