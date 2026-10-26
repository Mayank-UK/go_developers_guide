import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { ORMProbeDemoData } from './orm-probe';
import type { ORMProbeRepository } from './orm-probe-repository';
import { GetORMProbeDemoUseCase } from './get-orm-probe-demo-usecase';

/**
 * GetORMProbeDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

function fakeRepository(get: Result<ORMProbeDemoData>): ORMProbeRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: ORMProbeDemoData = {
  dataset: { posts: 12, tags: 5, edges: 24 },
  trials: [
    {
      strategy: 'pgx-baseline',
      label: 'Raw pgx: one JOIN',
      library: 'pgx v5',
      countingVia: 'pgx pool query tracer',
      statements: 1,
      roundTripRatio: '1x',
      rows: 12,
      durationMicros: 812,
      timingBand: 'sub-millisecond',
      sampleStatements: ['SELECT p.id, p.title … ORDER BY p.id'],
    },
    {
      strategy: 'gorm-lazy',
      label: 'GORM, the naive loop',
      library: 'GORM v1.31',
      countingVia: 'gorm session logger',
      statements: 13,
      roundTripRatio: '13x',
      rows: 12,
      durationMicros: 3241,
      timingBand: 'low-milliseconds',
      sampleStatements: [
        'SELECT * FROM "ormprobe_posts" ORDER BY id',
        'SELECT * FROM "ormprobe_tags" … "post_id" = 1',
      ],
    },
  ],
  resultsAgree: true,
  unitOfWork: {
    repositories: ['GormPostsRepository', 'GormTagsRepository'],
    steps: [
      { action: 'begin', detail: 'one unit of work' },
      { action: 'rollback', detail: 'deliberate failure' },
    ],
    hookApplied: true,
    postsBefore: 12,
    postsAfter: 12,
    rolledBack: true,
  },
};

describe('GetORMProbeDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetORMProbeDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetORMProbeDemoUseCase(
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
