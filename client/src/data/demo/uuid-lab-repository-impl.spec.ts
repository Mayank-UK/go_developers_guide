import { afterEach, describe, expect, it, vi } from 'vitest';
import { NetworkFailure, ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { UUIDLabApi } from './uuid-lab-api';
import { UUIDLabRepositoryImpl } from './uuid-lab-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * UUIDLabRepositoryImpl spec — fixture-driven (happy + failure): the
 * happy payload round-trips into the domain shape (VALUES are property-
 * checked, never literal-pinned — they are fresh randomness per load);
 * network / envelope / malformed-payload failures resolve as Failed
 * Results, never throws.
 */

const demoPayload = {
  generator: {
    name: 'UUIDv7, hand-rolled (RFC 9562 §5.7)',
    randomSource: 'crypto/rand — 12-bit counter seed + 62 random bits per id',
    description: 'no uuid library, no new dependency',
  },
  layout: [
    { field: 'unix_ts_ms', bits: 48, contents: 'Unix epoch milliseconds, big-endian' },
    { field: 'ver', bits: 4, contents: '0b0111 — version 7' },
    { field: 'rand_a', bits: 12, contents: 'a per-millisecond counter' },
    { field: 'var', bits: 2, contents: '0b10 — the RFC 4122/9562 variant marker' },
    { field: 'rand_b', bits: 62, contents: 'crypto/rand — uniqueness' },
  ],
  anatomy: {
    value: '0192ac15-0211-7abc-90d1-d2d3d4d5d6d7',
    unixMs: 1700000000000,
    version: 7,
    variant: 'RFC 4122/9562 (0b10)',
    length: 36,
  },
  batch: {
    count: 8,
    generatedInOrder: [
      '0192ac15-0211-7001-90d1-d2d3d4d5d6d7',
      '0192ac15-0211-7002-90d2-d2d3d4d5d6d7',
      '0192ac15-0211-7003-90d3-d2d3d4d5d6d7',
      '0192ac15-0211-7004-90d4-d2d3d4d5d6d7',
      '0192ac15-0211-7005-90d5-d2d3d4d5d6d7',
      '0192ac15-0211-7006-90d6-d2d3d4d5d6d7',
      '0192ac15-0211-7007-90d7-d2d3d4d5d6d7',
      '0192ac15-0211-7008-90d8-d2d3d4d5d6d7',
    ],
    sortedLexically: [
      '0192ac15-0211-7001-90d1-d2d3d4d5d6d7',
      '0192ac15-0211-7002-90d2-d2d3d4d5d6d7',
      '0192ac15-0211-7003-90d3-d2d3d4d5d6d7',
      '0192ac15-0211-7004-90d4-d2d3d4d5d6d7',
      '0192ac15-0211-7005-90d5-d2d3d4d5d6d7',
      '0192ac15-0211-7006-90d6-d2d3d4d5d6d7',
      '0192ac15-0211-7007-90d7-d2d3d4d5d6d7',
      '0192ac15-0211-7008-90d8-d2d3d4d5d6d7',
    ],
    sortingMatchesGenerationOrder: true,
  },
  collision: { generated: 10000, distinct: 10000, collisionFree: true, randomnessBits: 74 },
  comparison: [
    {
      strategy: 'bigserial',
      storage: '8 bytes',
      indexLocality: 'perfect — every insert appends to the btree\u2019s right edge',
      urlGuessability: 'enumerable — /posts/1, /posts/2, … invites scraping',
      uniquenessScope: 'one table\u2019s sequence',
      notes: 'cheap and ordered',
    },
  ],
};

function makeRepository(): UUIDLabRepositoryImpl {
  return new UUIDLabRepositoryImpl(new UUIDLabApi(createAppConfig()));
}

describe('UUIDLabRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload (properties, not literals)', async () => {
    stubApiFetch([{ url: '/api/demo/db/uuid', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.batch.count).toBe(8);
      expect(result.value.batch.sortingMatchesGenerationOrder).toBe(true);
      expect(result.value.collision.collisionFree).toBe(true);
      expect(result.value.collision.distinct).toBe(result.value.collision.generated);
      expect(result.value.anatomy.version).toBe(7);
      expect(result.value.layout).toHaveLength(5);
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result).toEqual(failed(new NetworkFailure('Could not reach the server.')));
    expect(result.ok).toBe(false);
  });

  it('guards an error envelope into a Failed Result with its diagnostics', async () => {
    stubApiFetch([
      {
        url: '/api/demo/db/uuid',
        status: 500,
        body: { error: { code: 'internal', message: 'mint failed', requestId: 'req-7' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(new ServerFailure('mint failed', { code: 'internal', status: 500, requestId: 'req-7' })),
    );
  });

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // Batch values that are not canonical UUIDv7 — the property check
      // must reject them.
      { url: '/api/demo/db/uuid', body: { ...demoPayload, batch: { count: 1, generatedInOrder: ['not-a-uuid'], sortedLexically: ['not-a-uuid'], sortingMatchesGenerationOrder: true } } },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('The server response did not match the expected shape.', {
          code: 'invalid_response',
          status: 200,
        }),
      ),
    );
  });
});
