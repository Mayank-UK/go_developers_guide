import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ORMProbeDemo } from './orm_probe_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetORMProbeDemoUseCase,
  DB_ORM_PROBE_DEMO_USE_CASE,
} from '../../../domain/demo/get-orm-probe-demo-usecase';
import type { ORMProbeDemoData } from '../../../domain/demo/orm-probe';

/**
 * ORMProbeDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration
 * instead of stubbing fetch. Behavior, not implementation: the dataset
 * chips render, every trial renders its measured statement count (the
 * naive loop visually flagged as the problem), statement receipts
 * render, the unit-of-work exhibit shows its rollback verdicts, and
 * failures — including the server's honest 503 for an unreachable
 * PostgreSQL — degrade to the shared error panel with recovery.
 */

const fixture: ORMProbeDemoData = {
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
      label: 'GORM, the naive loop: each post\u2019s tags fetched per row',
      library: 'GORM v1.31',
      countingVia: 'gorm session logger',
      statements: 13,
      roundTripRatio: '13x',
      rows: 12,
      durationMicros: 3241,
      timingBand: 'low-milliseconds',
      sampleStatements: [
        'SELECT * FROM "ormprobe_posts" ORDER BY id',
        'SELECT * FROM "ormprobe_tags" \u2026 WHERE "post_id" = 1',
        'SELECT * FROM "ormprobe_tags" \u2026 WHERE "post_id" = 2',
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
      sampleStatements: ['SELECT * FROM "ormprobe_post_tags" WHERE \u2026 IN (1,2,3)'],
    },
  ],
  resultsAgree: true,
  unitOfWork: {
    repositories: ['GormPostsRepository', 'GormTagsRepository'],
    steps: [
      { action: 'begin', detail: 'db.Transaction opens one unit of work over the probe\u2019s pool' },
      { action: 'read back inside the unit', detail: 'title stored as "unit-of-work scratch" — the hook trimmed it' },
      { action: 'rollback', detail: 'deliberate failure: prove the whole unit unwinds' },
      { action: 'count after', detail: '12 posts before, 12 after — nothing leaked' },
    ],
    hookApplied: true,
    postsBefore: 12,
    postsAfter: 12,
    rolledBack: true,
  },
};

function stubUseCase(execute: () => Promise<Result<ORMProbeDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    DB_ORM_PROBE_DEMO_USE_CASE,
    { execute } as unknown as GetORMProbeDemoUseCase,
  );
}

describe('ORMProbeDemo', () => {
  it('renders loading first, then the measured exhibit', async () => {
    stubUseCase(async () => success(fixture));

    render(<ORMProbeDemo />);

    expect(screen.getByTestId('orm-probe-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('orm-probe-ready')).toBeInTheDocument());

    const dataset = screen.getByTestId('op-dataset');
    expect(dataset).toHaveTextContent('12 posts');
    expect(dataset).toHaveTextContent('5 tags');
    expect(dataset).toHaveTextContent('24 edges');
    expect(dataset).toHaveTextContent('identical rows');
  });

  it('renders every trial with its measured statement count and receipts', async () => {
    stubUseCase(async () => success(fixture));

    render(<ORMProbeDemo />);

    await waitFor(() => expect(screen.getByTestId('op-trials')).toBeInTheDocument());

    const baseline = screen.getByTestId('op-trial-pgx-baseline');
    expect(baseline).toHaveTextContent('1 statements');
    expect(baseline).toHaveTextContent('pgx pool query tracer');

    const lazy = screen.getByTestId('op-trial-gorm-lazy');
    expect(lazy).toHaveTextContent('13 statements');
    expect(lazy).toHaveTextContent('13x the baseline');
    expect(lazy).toHaveTextContent('ormprobe_posts');
    expect(lazy).toHaveTextContent('… 10 more like it');

    const eager = screen.getByTestId('op-trial-gorm-eager');
    expect(eager).toHaveTextContent('3 statements');
    expect(eager).toHaveTextContent('3x the baseline');
  });

  it('renders the unit-of-work exhibit with its verdicts', async () => {
    stubUseCase(async () => success(fixture));

    render(<ORMProbeDemo />);

    const unit = await screen.findByTestId('op-unit-of-work');
    expect(unit).toHaveTextContent('GormPostsRepository');
    expect(unit).toHaveTextContent('GormTagsRepository');
    expect(unit).toHaveTextContent('✓ rolled back');
    expect(unit).toHaveTextContent('✓ BeforeCreate hook fired');
    expect(unit).toHaveTextContent('12 → 12 posts — nothing leaked');
    expect(unit).toHaveTextContent('deliberate failure');
  });

  it('degrades to the error panel on the server\u2019s honest 503, with recovery', async () => {
    const failure = new ServerFailure(
      'The ORM probe runs against the guide\u2019s live PostgreSQL, which is not reachable right now.',
      { code: 'service_unavailable', status: 503, requestId: 'req-orm-503' },
    );
    stubUseCase(async () => failed(failure));

    render(<ORMProbeDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');
    expect(panel).toHaveTextContent('not reachable right now');

    // Retry re-runs the measurement through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('orm-probe-ready')).toBeInTheDocument());
  });
});
