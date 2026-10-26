import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { UUIDLabDemoData } from './uuid-lab';
import type { UUIDLabRepository } from './uuid-lab-repository';
import { GetUUIDLabDemoUseCase } from './get-uuid-lab-demo-usecase';

/**
 * GetUUIDLabDemoUseCase spec — thin pass-through seam: whatever the
 * repository resolves arrives unchanged, and nothing throws. The payload
 * VALUES are fresh randomness per load (see uuid-lab.ts's header note) —
 * fixtures here pin the wire SHAPE, never minted uuid literals.
 */

function fakeRepository(get: Result<UUIDLabDemoData>): UUIDLabRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: UUIDLabDemoData = {
  generator: {
    name: 'UUIDv7, hand-rolled (RFC 9562 §5.7)',
    randomSource: 'crypto/rand — 12-bit counter seed + 62 random bits per id',
    description: '48-bit Unix milliseconds + version 7 + monotonic 12-bit counter',
  },
  layout: [
    { field: 'unix_ts_ms', bits: 48, contents: 'Unix epoch milliseconds, big-endian' },
    { field: 'ver', bits: 4, contents: '0b0111 — version 7' },
  ],
  anatomy: { value: '0192ac15-0211-7abc-90d1-d2d3d4d5d6d7', unixMs: 1700000000000, version: 7, variant: 'RFC 4122/9562 (0b10)', length: 36 },
  batch: {
    count: 8,
    generatedInOrder: [
      '0192ac15-0211-7001-90d1-d2d3d4d5d6d7',
      '0192ac15-0211-7002-90d1-d2d3d4d5d6d7',
    ],
    sortedLexically: [
      '0192ac15-0211-7001-90d1-d2d3d4d5d6d7',
      '0192ac15-0211-7002-90d1-d2d3d4d5d6d7',
    ],
    sortingMatchesGenerationOrder: true,
  },
  collision: { generated: 10000, distinct: 10000, collisionFree: true, randomnessBits: 74 },
  comparison: [
    {
      strategy: 'bigserial',
      storage: '8 bytes',
      indexLocality: 'perfect',
      urlGuessability: 'enumerable',
      uniquenessScope: "one table's sequence",
      notes: 'cheap and ordered, but ids leak volume and pace',
    },
  ],
};

describe('GetUUIDLabDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetUUIDLabDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetUUIDLabDemoUseCase(
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
