import { describe, expect, it } from 'vitest';
import { ormProbeDtoFromJson } from './orm-probe-dto';

/**
 * ormProbeDtoFromJson spec — pinned against the REAL payload shape the
 * server's BuildORMProbePayload emits (camelCase, four trials in the
 * canonical order, statement receipts included). The shape is copied
 * from the handler test's decoded fixture, not invented.
 */

const payload = {
  dataset: { posts: 12, tags: 5, edges: 24 },
  trials: [
    {
      strategy: 'pgx-baseline',
      label: 'Raw pgx: one JOIN, tags aggregated in the same round trip',
      library: 'pgx v5',
      countingVia: 'pgx pool query tracer',
      statements: 1,
      roundTripRatio: '1x',
      rows: 12,
      durationMicros: 812,
      timingBand: 'sub-millisecond',
      sampleStatements: ['SELECT p.id, p.title, p.body, p.created_at, … ORDER BY p.id'],
    },
    {
      strategy: 'sqlc-join',
      label: 'sqlc: the same JOIN as generated, typed code',
      library: 'sqlc v1.31 + pgx v5',
      countingVia: 'explicit generated call sites',
      statements: 1,
      roundTripRatio: '1x',
      rows: 12,
      durationMicros: 903,
      timingBand: 'sub-millisecond',
      sampleStatements: ['-- name: ListPostsWithTags :many SELECT p.id, …'],
    },
    {
      strategy: 'gorm-lazy',
      label: 'GORM, the naive loop: each post’s tags fetched per row',
      library: 'GORM v1.31',
      countingVia: 'gorm session logger',
      statements: 13,
      roundTripRatio: '13x',
      rows: 12,
      durationMicros: 3241,
      timingBand: 'low-milliseconds',
      sampleStatements: [
        'SELECT * FROM "ormprobe_posts" ORDER BY id',
        'SELECT * FROM "ormprobe_tags" … WHERE "post_id" = 1',
        'SELECT * FROM "ormprobe_tags" … WHERE "post_id" = 2',
      ],
    },
    {
      strategy: 'gorm-eager',
      label: 'GORM with Preload: the page, then one IN query for all tags',
      library: 'GORM v1.31',
      countingVia: 'gorm session logger',
      statements: 3,
      roundTripRatio: '3x',
      rows: 12,
      durationMicros: 1187,
      timingBand: 'sub-millisecond',
      sampleStatements: ['SELECT * FROM "ormprobe_post_tags" WHERE … IN (1,2,3)'],
    },
  ],
  resultsAgree: true,
  unitOfWork: {
    repositories: ['GormPostsRepository', 'GormTagsRepository'],
    steps: [
      { action: 'begin', detail: 'db.Transaction opens one unit of work over the probe’s pool' },
      { action: 'rollback', detail: 'deliberate failure: prove the whole unit unwinds' },
    ],
    hookApplied: true,
    postsBefore: 12,
    postsAfter: 12,
    rolledBack: true,
  },
};

describe('ormProbeDtoFromJson', () => {
  it('builds the DTO from the real payload shape', () => {
    const dto = ormProbeDtoFromJson(payload);

    expect(dto.dataset).toEqual({ posts: 12, tags: 5, edges: 24 });
    expect(dto.trials).toHaveLength(4);
    expect(dto.trials[2]).toMatchObject({
      strategy: 'gorm-lazy',
      statements: 13,
      roundTripRatio: '13x',
      timingBand: 'low-milliseconds',
    });
    expect(dto.trials[2]?.sampleStatements).toHaveLength(3);
    expect(dto.resultsAgree).toBe(true);
    expect(dto.unitOfWork).toMatchObject({
      hookApplied: true,
      rolledBack: true,
      postsBefore: 12,
      postsAfter: 12,
    });
  });

  it('refuses malformed payloads', () => {
    expect(() => ormProbeDtoFromJson('x')).toThrow();
    expect(() => ormProbeDtoFromJson({ ...payload, trials: [{}] })).toThrow();
    expect(() => ormProbeDtoFromJson({ ...payload, dataset: { posts: 'twelve' } })).toThrow();
    expect(() => ormProbeDtoFromJson({ ...payload, trials: [{ ...payload.trials[0], strategy: 'mystery' }] })).toThrow();
    expect(() => ormProbeDtoFromJson({ ...payload, unitOfWork: { ...payload.unitOfWork, rolledBack: 'yes' } })).toThrow();
  });

  it('accepts present-but-empty explanatory prose (requiredText semantics)', () => {
    const quiet = {
      ...payload,
      trials: [{ ...payload.trials[0], label: '', countingVia: '' }],
    };

    const dto = ormProbeDtoFromJson(quiet);

    expect(dto.trials[0]?.label).toBe('');
    expect(dto.trials[0]?.countingVia).toBe('');
  });
});
