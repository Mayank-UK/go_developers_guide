import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetArchTracerDemoUseCase,
  ARCH_TRACER_DEMO_USE_CASE,
} from '../../../domain/demo/get-arch-tracer-demo-usecase';
import type { ArchTracerDemoData, ArchTraceHop } from '../../../domain/demo/arch-tracer';

/**
 * ArchTracerDemo — the architecture topic's live demo (demo block id
 * `arch:tracer`). It renders the payload of GET /api/demo/arch/tracer:
 * a REAL posts-list request the server walked through its actual
 * layers — middleware → thin handler → use case → the adapter the
 * scenario picked — every hop labeled by the file that ran it and
 * timed inclusively at its seam, drawn as a timeline with proportional
 * duration bars. The leaf row names the store that answered and how to
 * flip it (the scenario endpoint — reload after flipping re-runs the
 * same trace with the other leaf). Beside the timeline, the
 * dependency-rule counterexample: what a handler importing the data
 * layer would look like, clearly labeled as non-compiling data.
 */

type TracerStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ArchTracerDemoData }
  | { state: 'error'; failure: Failure };

export function useArchTracerDemo(
  useCase: GetArchTracerDemoUseCase = container.resolve(ARCH_TRACER_DEMO_USE_CASE),
): { status: TracerStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<TracerStatus>({ state: 'idle' });

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

/** The longest hop anchors the timeline's scale; every bar is relative. */
function barPercent(durationMs: number, maxMs: number): string {
  if (maxMs <= 0) return '0%';
  return `${Math.max(2, Math.round((durationMs / maxMs) * 100))}%`;
}

function layerClass(layer: ArchTraceHop['layer']): string {
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

export function ArchTracerDemo() {
  const { status, retry } = useArchTracerDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="arch-tracer-loading" className="text-body-sm text-on-surface-variant">
        walking one request through the layers…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the architecture tracer demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;
  const { trace } = data;
  const maxMs = Math.max(...trace.hops.map((hop) => hop.durationMs), 0);

  return (
    <div className="flex flex-col gap-md" data-testid="arch-tracer-ready">
      <section
        data-testid="arch-tracer-timeline"
        className="flex flex-col gap-sm rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          <span className="text-secondary">{trace.request}</span> — mode {trace.dataMode} → answered{' '}
          {String(trace.status)} with {String(trace.items)} items in {String(trace.elapsedMs)} ms
        </p>
        <p className="font-mono text-caption text-on-surface">
          leaf: <span className="text-secondary">{trace.leaf}</span>
        </p>
        {trace.hops.map((hop) => (
          <div key={hop.order} data-testid={`arch-tracer-hop-${String(hop.order)}`} className="flex flex-col gap-xxs">
            <p className="font-mono text-caption">
              <span className={layerClass(hop.layer)}>{hop.layer}</span>{' '}
              <span className="text-on-surface-variant">{hop.file}</span> · {hop.symbol} ·{' '}
              {String(hop.durationMs)} ms
            </p>
            <div className="h-xxs rounded-sm bg-outline" style={{ width: '100%' }}>
              <div
                className={`h-xxs rounded-sm ${hop.layer === 'data' ? 'bg-secondary' : 'bg-primary'}`}
                style={{ width: barPercent(hop.durationMs, maxMs) }}
              />
            </div>
            <p className="text-caption text-on-surface-variant">{hop.note}</p>
          </div>
        ))}
        <p className="text-caption text-on-surface">{trace.note}</p>
        <p className="text-caption text-on-surface-variant">{trace.flipNote}</p>
      </section>

      <section
        data-testid="arch-tracer-violation"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface-variant">{data.violation.label}</p>
        <pre className="overflow-x-auto rounded-sm bg-surface-tint px-sm py-xxs font-mono text-caption text-on-surface-variant">
          {data.violation.snippet}
        </pre>
        <p className="text-caption text-on-surface">{data.violation.why}</p>
        <p className="text-caption text-on-surface-variant">{data.violation.caughtBy}</p>
        <p className="text-caption text-on-surface">{data.violation.honest}</p>
      </section>

      <p data-testid="arch-tracer-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

export default ArchTracerDemo;
