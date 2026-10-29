import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import { ScenarioPanel } from '../../components/scenario-panel';
import type {
  ReliabilityBreakerDemoData,
  ReliabilityRetryRowView,
  ReliabilityAttemptView,
} from '../../../domain/demo/reliability-breaker';
import {
  GetReliabilityBreakerDemoUseCase,
  RELIABILITY_BREAKER_DEMO_USE_CASE,
} from '../../../domain/demo/get-reliability-breaker-demo-usecase';

/**
 * ReliabilityBreakerDemo — the reliability topic's breaker demo (demo
 * block id `reliability:breaker`). It renders the payload of
 * GET /api/demo/reliability/breaker: the armed scenario toggle, the
 * PERSISTENT circuit breaker (state with its age, config, counters,
 * and the transition ring), this request's walk — attempt by attempt,
 * with the state before/after, the action (call or fast-fail), the
 * outcome, and the MEASURED elapsed time — and the bounded
 * retry-with-backoff series beside it. The scenario panel embedded at
 * the top is this demo's own control (it writes PATCH /api/demo/scenario):
 * flip it and re-walk to trip the breaker under error injection, convert
 * stalls into deadline exceeded under latency, and let the half-open
 * probe close the circuit once disarmed. The client never teaches; it
 * renders what the server measured.
 */

type BreakerStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ReliabilityBreakerDemoData }
  | { state: 'error'; failure: Failure };

/** formatMs renders a measured millisecond figure with sub-ms honesty. */
function formatMs(ms: number): string {
  return ms >= 100 ? `${Math.round(ms)}ms` : `${ms.toFixed(2)}ms`;
}

/** formatDurationMs renders config-panel durations readably. */
function formatDurationMs(ms: number): string {
  return ms >= 1000 ? `${(ms / 1000).toFixed(ms % 1000 === 0 ? 0 : 1)}s` : `${Math.round(ms)}ms`;
}

/** One walk attempt: the state journey and what it cost, measured. */
function AttemptLine({ row }: { row: ReliabilityAttemptView }) {
  const refused = row.action === 'fast-fail';
  return (
    <p className="font-mono text-caption">
      <span className="text-on-surface">
        #{row.index} {row.stateBefore} → {row.stateAfter}
      </span>
      <span className={refused ? 'text-on-surface-variant' : 'text-on-surface'}>
        {' '}
        {row.action} · {row.outcome}
      </span>
      <span className="text-on-surface-variant"> · {formatMs(row.elapsedMs)}</span>
      {row.detail ? <span className="text-on-surface-variant"> — {row.detail}</span> : null}
    </p>
  );
}

/** One retry row: outcome, measured attempt, measured backoff wait. */
function RetryLine({ row }: { row: ReliabilityRetryRowView }) {
  return (
    <p className="font-mono text-caption">
      <span className="text-on-surface">retry {row.attempt} · {row.outcome}</span>
      <span className="text-on-surface-variant">
        {' '}
        · {formatMs(row.elapsedMs)}
        {row.backoffMs > 0 ? ` · wait ${formatMs(row.backoffMs)}` : ' · final attempt'}
      </span>
    </p>
  );
}

export function useReliabilityBreakerDemo(
  useCase: GetReliabilityBreakerDemoUseCase = container.resolve(RELIABILITY_BREAKER_DEMO_USE_CASE),
): { status: BreakerStatus; reload: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<BreakerStatus>({ state: 'idle' });

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

  const reload = () => setAttempt((current) => current + 1);
  return { status, reload };
}

export function ReliabilityBreakerDemo() {
  const { status, reload } = useReliabilityBreakerDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="reliability-breaker-loading" className="text-body-sm text-on-surface-variant">
        walking the breaker…
      </p>
    );
  }

  if (status.state === 'error') {
    return (
      <ErrorPanel subject="the reliability breaker demo" failure={status.failure} onRetry={reload} />
    );
  }

  const { data } = status;
  const breaker = data.breaker;
  const config = breaker.config;

  return (
    <div className="flex flex-col gap-md" data-testid="reliability-breaker-ready">
      <section
        data-testid="reliability-breaker-controls"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        {/* The demo's own scenario control — the shared fake/live +
            failure-injection toggle, embedded so this page drives standalone. */}
        <ScenarioPanel />
      </section>

      <section
        data-testid="reliability-breaker-scenario"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          scenario · {data.scenario.mode} — {data.scenario.patchEndpoint} flips it (
          {data.scenario.modes.join(' | ')})
        </p>
      </section>

      <section
        data-testid="reliability-breaker-state"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          breaker · {breaker.state} · held for {formatDurationMs(breaker.stateAgeMs)}
          {breaker.probeInFlight ? ' · probe in flight' : ''}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          threshold {config.failureThreshold} · cooldown {formatDurationMs(config.cooldownMs)} ·
          deadline {formatDurationMs(config.callTimeoutMs)} · {config.halfOpenProbes} half-open
          probe · retry {config.retryAttempts}× from {formatDurationMs(config.retryBaseDelayMs)} +
          ≤{formatDurationMs(config.retryJitterMs)} jitter
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          allowed {breaker.counters.allowed} · succeeded {breaker.counters.succeeded} · failed{' '}
          {breaker.counters.failed} · fast-failed {breaker.counters.fastFailed}
        </p>
        {breaker.transitions.length > 0 ? (
          breaker.transitions
            .slice(-3)
            .reverse()
            .map((transition) => (
              <p
                key={transition.at + transition.to}
                className="font-mono text-caption text-on-surface-variant"
              >
                <span className="text-on-surface">
                  {transition.from} → {transition.to}
                </span>{' '}
                — {transition.cause}
              </p>
            ))
        ) : (
          <p className="font-mono text-caption text-on-surface-variant">
            no state changes yet — the circuit has stayed closed since boot
          </p>
        )}
      </section>

      <section
        data-testid="reliability-breaker-walk"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.walk.rows.map((row) => (
          <AttemptLine key={row.index} row={row} />
        ))}
        <p className="text-caption text-on-surface-variant">{data.walk.note}</p>
      </section>

      <section
        data-testid="reliability-breaker-retry"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          retry series · {data.retry.succeeded ? 'succeeded' : 'exhausted'} · total{' '}
          {formatMs(data.retry.totalElapsedMs)}
        </p>
        {data.retry.rows.map((row) => (
          <RetryLine key={row.attempt} row={row} />
        ))}
        <p className="text-caption text-on-surface-variant">{data.retry.note}</p>
      </section>

      <p data-testid="reliability-breaker-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>

      <button
        type="button"
        onClick={reload}
        data-testid="reliability-breaker-rewalk"
        className="self-start rounded-md border border-outline px-md py-xxs text-caption text-on-surface hover:bg-surface-tint"
      >
        walk the breaker again
      </button>
    </div>
  );
}

export default ReliabilityBreakerDemo;
