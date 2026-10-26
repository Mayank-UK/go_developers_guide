import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetORMProbeDemoUseCase,
  DB_ORM_PROBE_DEMO_USE_CASE,
} from '../../../domain/demo/get-orm-probe-demo-usecase';
import type { ORMProbeDemoData, ORMProbeTrial } from '../../../domain/demo/orm-probe';

/**
 * ORMProbeDemo — the sqlc-gorm topic's live demo (demo block id
 * `db:orm-probe`). It renders the payload of GET
 * /api/demo/db/orm-probe: one seeded dataset, the same logical job run
 * four ways (raw pgx, sqlc, GORM naive loop, GORM Preload), every SQL
 * statement counted through its library's own seam, and the
 * two-repository unit-of-work exhibit that provably rolls back.
 *
 * Statement counts, rows, and ratios are durable facts; the timing band
 * is the live observation of the run that served this response — a
 * range, never a promise. An unreachable database degrades to the typed
 * error panel (the server's honest 503), never to a stale answer.
 */

type ORMProbeStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ORMProbeDemoData }
  | { state: 'error'; failure: Failure };

export function useORMProbeDemo(
  useCase: GetORMProbeDemoUseCase = container.resolve(DB_ORM_PROBE_DEMO_USE_CASE),
): { status: ORMProbeStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ORMProbeStatus>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });

    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus({ state: 'ready', data: result.value });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function ORMProbeDemo() {
  const { status, retry } = useORMProbeDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="orm-probe-loading" className="text-body-sm text-on-surface-variant">
        running the probe against the live PostgreSQL…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the ORM probe" failure={status.failure} onRetry={retry} />;
  }

  const { dataset, trials, resultsAgree, unitOfWork } = status.data;

  return (
    <div className="flex flex-col gap-md" data-testid="orm-probe-ready">
      <section data-testid="op-dataset" className="flex flex-wrap gap-xs">
        <span className="rounded border border-outline px-sm py-xxs font-mono text-caption text-on-surface">
          {dataset.posts} posts
        </span>
        <span className="rounded border border-outline px-sm py-xxs font-mono text-caption text-on-surface">
          {dataset.tags} tags
        </span>
        <span className="rounded border border-outline px-sm py-xxs font-mono text-caption text-on-surface">
          {dataset.edges} edges
        </span>
        <span
          className={`rounded px-sm py-xxs font-mono text-caption ${resultsAgree ? 'bg-secondary text-surface' : 'bg-surface-tint text-error'}`}
        >
          {resultsAgree ? '✓ all four trials answered identical rows' : '✗ trials disagreed'}
        </span>
      </section>

      <section data-testid="op-trials" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          One job, four ways — SQL statements counted
        </h4>
        <div className="flex flex-col gap-sm">
          {trials.map((trial) => (
            <TrialCard key={trial.strategy} trial={trial} />
          ))}
        </div>
        <p className="text-caption text-on-surface-variant">
          Counts are the durable facts; the timing band is what THIS run observed — a range,
          not a promise.
        </p>
      </section>

      <section
        data-testid="op-unit-of-work"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">
          Two repositories, one transaction, one deliberate failure
        </h4>
        <div className="flex flex-wrap gap-xxs">
          {unitOfWork.repositories.map((name) => (
            <span
              key={name}
              className="rounded border border-outline px-xs py-xxs font-mono text-caption text-on-surface-variant"
            >
              {name}
            </span>
          ))}
        </div>
        <ol className="flex flex-col gap-xxs">
          {unitOfWork.steps.map((step, index) => (
            <li key={`${step.action}-${index}`} className="text-body-sm text-on-surface-variant">
              <span className="font-mono text-on-surface">{step.action}</span> — {step.detail}
            </li>
          ))}
        </ol>
        <div className="flex flex-wrap gap-xxs">
          <span
            className={`rounded px-xs py-xxs font-mono text-caption ${unitOfWork.rolledBack ? 'bg-secondary text-surface' : 'bg-surface-tint text-error'}`}
          >
            {unitOfWork.rolledBack ? '✓ rolled back' : '✗ committed (unexpected)'}
          </span>
          <span
            className={`rounded px-xs py-xxs font-mono text-caption ${unitOfWork.hookApplied ? 'bg-secondary text-surface' : 'bg-surface-tint text-error'}`}
          >
            {unitOfWork.hookApplied ? '✓ BeforeCreate hook fired' : '✗ hook silent'}
          </span>
          <span className="rounded border border-outline px-xs py-xxs font-mono text-caption text-on-surface-variant">
            {unitOfWork.postsBefore} → {unitOfWork.postsAfter} posts — nothing leaked
          </span>
        </div>
      </section>
    </div>
  );
}

function TrialCard({ trial }: { trial: ORMProbeTrial }) {
  const isTheProblem = trial.strategy === 'gorm-lazy';
  return (
    <div
      className={`flex flex-col gap-xs rounded-md border px-md py-sm ${isTheProblem ? 'border-error' : 'border-outline'}`}
      data-testid={`op-trial-${trial.strategy}`}
    >
      <div className="flex flex-wrap items-baseline justify-between gap-xs">
        <h5 className="text-body-sm text-on-surface">{trial.label}</h5>
        <span className="font-mono text-caption text-secondary">{trial.library}</span>
      </div>
      <div className="flex flex-wrap items-baseline gap-xs">
        <span
          className={`font-mono text-title-sm ${isTheProblem ? 'text-error' : 'text-on-surface'}`}
        >
          {trial.statements} statements
        </span>
        <span className="rounded bg-surface-tint px-xs py-xxs font-mono text-caption text-secondary">
          {trial.roundTripRatio} the baseline
        </span>
        <span className="font-mono text-caption text-on-surface-variant">
          {trial.rows} rows · {trial.timingBand} · counted via {trial.countingVia}
        </span>
      </div>
      {trial.sampleStatements.length > 0 ? (
        <div className="flex flex-col gap-xxs">
          {trial.sampleStatements.map((statement, index) => (
            <code
              key={index}
              className="block overflow-x-auto rounded bg-surface-tint px-xs py-xxs font-mono text-caption text-on-surface-variant"
            >
              {statement}
            </code>
          ))}
          {trial.statements > trial.sampleStatements.length ? (
            <span className="font-mono text-caption text-on-surface-variant">
              … {trial.statements - trial.sampleStatements.length} more like it
            </span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

export default ORMProbeDemo;
