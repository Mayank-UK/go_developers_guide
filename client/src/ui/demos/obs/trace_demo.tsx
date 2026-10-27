import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetObsTraceDemoUseCase,
  OBS_TRACE_DEMO_USE_CASE,
} from '../../../domain/demo/get-obs-trace-demo-usecase';
import type { ObsTraceDemoData } from '../../../domain/demo/obs-trace';

/**
 * ObsTraceDemo — the observability topic's trace round-trip demo (demo
 * block id `obs:trace`). It renders the payload of
 * GET /api/demo/obs/trace: the acceptance panel (the inbound
 * traceparent — adopted when the request carried one, minted when it
 * did not — plus the parsed fields and the response echo), the span
 * tree (middleware → thin handler → use case/port → adapter, indented
 * by depth, the root's parent being the inbound remote span id), the
 * hostile-header table with the parser's real rejection reasons, and
 * the honest OpenTelemetry SDK comparison.
 */

type TraceStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ObsTraceDemoData }
  | { state: 'error'; failure: Failure };

export function useObsTraceDemo(
  useCase: GetObsTraceDemoUseCase = container.resolve(OBS_TRACE_DEMO_USE_CASE),
): { status: TraceStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<TraceStatus>({ state: 'idle' });

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

/** The longest span anchors the tree's bar scale; every bar is relative. */
function barPercent(durationMs: number, maxMs: number): string {
  if (maxMs <= 0) return '0%';
  return `${Math.max(2, Math.round((durationMs / maxMs) * 100))}%`;
}

function layerClass(layer: string): string {
  switch (layer) {
    case 'presentation':
      return 'text-primary';
    case 'domain':
      return 'text-secondary';
    case 'data':
      return 'text-on-surface';
    default:
      return 'text-on-surface-variant';
  }
}

export function ObsTraceDemo() {
  const { status, retry } = useObsTraceDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="obs-trace-loading" className="text-body-sm text-on-surface-variant">
        walking one traced request through the layers…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the trace round-trip demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;
  const maxMs = Math.max(...data.tree.map((span) => span.durationMs), 0);

  return (
    <div className="flex flex-col gap-md" data-testid="obs-trace-ready">
      <section
        data-testid="obs-trace-accepted"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          inbound: <span className="text-secondary">{data.accepted.inbound || '(none sent)'}</span>
        </p>
        <p className="text-caption text-on-surface-variant">{data.accepted.inboundOrigin}</p>
        <p className="font-mono text-caption">
          trace <span className="text-secondary">{data.accepted.traceId}</span> · parent{' '}
          <span className="text-on-surface">{data.accepted.parentId}</span> · flags{' '}
          <span className="text-on-surface">{data.accepted.flags}</span>
          {data.accepted.sampled ? ' (sampled)' : ' (not sampled)'}
        </p>
        {data.accepted.tracestate ? (
          <p className="font-mono text-caption text-on-surface-variant">
            tracestate {data.accepted.tracestate}
          </p>
        ) : null}
        <p className="font-mono text-caption text-on-surface">
          echo: <span className="text-secondary">{data.accepted.responseEcho}</span>
        </p>
        <p className="text-caption text-on-surface-variant">{data.accepted.note}</p>
      </section>

      <section
        data-testid="obs-trace-tree"
        className="flex flex-col gap-sm rounded-md border border-outline px-md py-sm"
      >
        {data.tree.map((span, index) => (
          <div
            key={span.spanId}
            data-testid={`obs-trace-span-${String(index + 1)}`}
            className="flex flex-col gap-xxs"
            style={{ marginLeft: `${span.depth * 1.25}rem` }}
          >
            <p className="font-mono text-caption">
              <span className={layerClass(span.layer)}>{span.layer}</span>{' '}
              <span className="text-on-surface">{span.name}</span> · {String(span.durationMs)} ms
            </p>
            <div className="h-xxs rounded-sm bg-outline" style={{ width: '100%' }}>
              <div
                className={`h-xxs rounded-sm ${span.layer === 'data' ? 'bg-secondary' : 'bg-primary'}`}
                style={{ width: barPercent(span.durationMs, maxMs) }}
              />
            </div>
            <p className="text-caption text-on-surface-variant">{span.note}</p>
          </div>
        ))}
      </section>

      <section
        data-testid="obs-trace-malformed"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.malformed.map((row) => (
          <p key={row.header} className="font-mono text-caption text-on-surface-variant">
            <span className="text-on-surface">{row.header}</span> → {row.problem}
          </p>
        ))}
        <p className="text-caption text-on-surface-variant">{data.malformed[0]?.handledAs}</p>
      </section>

      <p data-testid="obs-trace-sdk" className="text-caption text-on-surface">
        {data.sdk}
      </p>
      <p data-testid="obs-trace-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

export default ObsTraceDemo;
