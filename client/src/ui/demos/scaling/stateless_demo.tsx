import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type {
  ScalingStatelessDemoData,
  ScalingStepView,
} from '../../../domain/demo/scaling-stateless';
import {
  GetScalingStatelessDemoUseCase,
  SCALING_STATELESS_DEMO_USE_CASE,
} from '../../../domain/demo/get-scaling-stateless-demo-usecase';

/**
 * ScalingStatelessDemo — the scaling topic's statelessness demo (demo
 * block id `scaling:stateless`). It renders the payload of
 * GET /api/demo/scaling/stateless: the round's config (three replicas,
 * twelve distribution requests, the two session tokens), the MEASURED
 * distribution across the fleet, the thirteen-step session round —
 * the local session stored once and missed on every other hop,
 * sticky routing's rescue and its death at the rolling-update swap,
 * the shared session found by every replica including fresh
 * replacements — the counter beat, and the verdicts derived from the
 * measured numbers. The round is deterministic, so the replay button
 * re-runs an identical choreography: the lesson replays, the numbers
 * do not drift. The client never teaches; it renders what the server
 * measured.
 */

type RoundStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ScalingStatelessDemoData }
  | { state: 'error'; failure: Failure };

/** phaseTone colors a step by its placement story. */
function phaseTone(phase: string): string {
  if (phase === 'shared-store') return 'text-primary';
  if (phase === 'swap') return 'text-error';
  if (phase === 'sticky') return 'text-on-surface-variant';
  return 'text-on-surface';
}

/** One session-round step: phase, routing, outcome, and the measured observation. */
function StepLine({ row }: { row: ScalingStepView }) {
  return (
    <p className={`font-mono text-caption ${phaseTone(row.phase)}`} data-testid={`scl-step-${row.step}`}>
      <span className="text-on-surface">
        {row.step}. [{row.phase}] {row.replicaId}
      </span>{' '}
      {row.label} — {row.observed}
    </p>
  );
}

export function useScalingStatelessDemo(
  useCase: GetScalingStatelessDemoUseCase = container.resolve(SCALING_STATELESS_DEMO_USE_CASE),
): { status: RoundStatus; replay: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<RoundStatus>({ state: 'idle' });

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

  const replay = () => setAttempt((current) => current + 1);
  return { status, replay };
}

export function ScalingStatelessDemo() {
  const { status, replay } = useScalingStatelessDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="scaling-stateless-loading" className="text-body-sm text-on-surface-variant">
        walking one session through the fleet…
      </p>
    );
  }

  if (status.state === 'error') {
    return (
      <ErrorPanel subject="the statelessness round" failure={status.failure} onRetry={replay} />
    );
  }

  const { data } = status;
  const config = data.config;

  return (
    <div className="flex flex-col gap-md" data-testid="scaling-stateless-ready">
      <section
        data-testid="scaling-stateless-config"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          fleet · {config.replicas} replicas · {config.distributionRequests} distribution requests ·
          client {config.clientId} · {config.counterIncrements} increments
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          tokens · {config.localToken} (replica-local) vs {config.sharedToken} (shared store) —{' '}
          {config.sharedStoreNote}
        </p>
      </section>

      <section
        data-testid="scaling-stateless-distribution"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          distribution · {data.distribution.totalServed} requests ·{' '}
          {data.distribution.rows.map((row) => `${row.replicaId} ${row.served}`).join(' · ')}
        </p>
        <p className="text-caption text-on-surface-variant">{data.distribution.note}</p>
      </section>

      <section
        data-testid="scaling-stateless-steps"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.steps.map((row) => (
          <StepLine key={row.step} row={row} />
        ))}
      </section>

      <section
        data-testid="scaling-stateless-counter"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          counter ·{' '}
          {data.counter.rows.map((row) => `${row.replicaId} +${row.increments}`).join(' · ')} ·
          shared reads {data.counter.total}
        </p>
        <p className="text-caption text-on-surface-variant">{data.counter.note}</p>
      </section>

      <section
        data-testid="scaling-stateless-verdicts"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.verdicts.map((verdict) => (
          <p key={verdict.id} className="font-mono text-caption text-on-surface" data-testid={`scl-verdict-${verdict.id}`}>
            <span className="text-on-surface-variant">{verdict.label}:</span> {verdict.value}
          </p>
        ))}
      </section>

      <p data-testid="scaling-stateless-honesty" className="text-body-sm text-on-surface-variant">
        {data.honesty}
      </p>

      <button
        type="button"
        onClick={replay}
        data-testid="scaling-stateless-replay"
        className="self-start rounded-md border border-outline px-md py-xxs text-caption text-on-surface hover:bg-surface-tint"
      >
        replay the round
      </button>
    </div>
  );
}

export default ScalingStatelessDemo;
