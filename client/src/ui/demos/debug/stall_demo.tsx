import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { DebugStallStack } from '../../../domain/demo/debug-stall';
import {
  GetDebugStallDemoUseCase,
  DEBUG_STALL_DEMO_USE_CASE,
} from '../../../domain/demo/get-debug-stall-demo-usecase';
import type { DebugStallDemoData } from '../../../domain/demo/debug-stall';

/**
 * DebugStallDemo — the debugging topic's stall demo (demo block id
 * `debug:stall`). It renders the payload of
 * GET /api/demo/debug/stall: the workload choreography (five workers,
 * four healthy, one wedged), the capture's provenance (profile name,
 * goroutines seen, cost, attempts, the equivalent curl), the STALLED
 * stack (id, state, wait, frames with source lines, the created-by
 * chain) beside the CONTRAST row (the request's own goroutine from the
 * same capture), and the live pprof endpoints for the reader's own
 * exploration. Every GET stages a fresh stall — the reload gesture is
 * "capture again".
 */

type StallStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: DebugStallDemoData }
  | { state: 'error'; failure: Failure };

export function useDebugStallDemo(
  useCase: GetDebugStallDemoUseCase = container.resolve(DEBUG_STALL_DEMO_USE_CASE),
): { status: StallStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<StallStatus>({ state: 'idle' });

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

function StackRow({ label, stack }: { label: string; stack: DebugStallStack }) {
  const waitLabel = stack.wait ? ` · ${stack.wait}` : '';
  return (
    <div className="flex flex-col gap-xxs">
      <p className="font-mono text-caption text-on-surface">
        {label} · goroutine {String(stack.id)}{' '}
        <span className="text-on-surface-variant">[{stack.state}{waitLabel}]</span>
      </p>
      {stack.frames.map((frame) => (
        <div key={`${frame.function}:${frame.location}`} className="flex flex-col gap-xxs">
          <p className="font-mono text-caption text-on-surface-variant">{frame.function}</p>
          {frame.location ? (
            <p
              className="font-mono text-caption text-on-surface-variant"
              style={{ marginLeft: '1rem' }}
            >
              {frame.location}
            </p>
          ) : null}
        </div>
      ))}
      {stack.createdBy ? (
        <p className="font-mono text-caption text-on-surface-variant">
          created by {stack.createdBy}
        </p>
      ) : null}
    </div>
  );
}

export function DebugStallDemo() {
  const { status, retry } = useDebugStallDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="debug-stall-loading" className="text-body-sm text-on-surface-variant">
        staging a fresh wedge and capturing its anatomy…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the stall demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="debug-stall-ready">
      <section
        data-testid="debug-stall-workload"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          {String(data.workload.healthyDone)}/{String(data.workload.workers)} workers completed ·
          one wedged
        </p>
        <p className="text-caption text-on-surface-variant">{data.workload.wedge}</p>
      </section>

      <section
        data-testid="debug-stall-capture"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          {data.capture.profile} profile · {String(data.capture.goroutinesSeen)} goroutines seen ·{' '}
          {String(Math.round(data.capture.captureMs * 10) / 10)}ms ·{' '}
          {String(data.capture.captureTries)} capture try
        </p>
        <p className="font-mono text-caption text-on-surface-variant">{data.capture.curl}</p>
      </section>

      <section
        data-testid="debug-stall-stalled"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <StackRow label="stalled" stack={data.stalled} />
      </section>

      <section
        data-testid="debug-stall-contrast"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <StackRow label="contrast" stack={data.contrast} />
      </section>

      <section
        data-testid="debug-stall-endpoints"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.endpoints.map((endpoint) => (
          <p key={endpoint.path} className="font-mono text-caption">
            <span className="text-on-surface">{endpoint.path}</span>{' '}
            <span className="text-on-surface-variant">— {endpoint.note}</span>
          </p>
        ))}
      </section>

      <p data-testid="debug-stall-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

export default DebugStallDemo;
