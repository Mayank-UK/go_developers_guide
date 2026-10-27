import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetErrorsExplorerDemoUseCase,
  ERRORS_EXPLORER_DEMO_USE_CASE,
} from '../../../domain/demo/get-errors-explorer-demo-usecase';
import type {
  ErrorsExplorerDemoData,
  ErrorsKindRow,
} from '../../../domain/demo/errors-explorer';

/**
 * ErrorsExplorerDemo — the error-handling topic's live demo (demo block
 * id `errors:explorer`). It renders the payload of GET
 * /api/demo/errors/explorer: the eight-kind taxonomy table (statuses
 * and messages derived from the server's own core tables, every row
 * carrying its raiser, its log line, and its curl-able trigger — or the
 * honest recipe when the running stack cannot produce that failure),
 * the scenario and probe panels, and the LIVE walk: the status, body,
 * and request id of the probe the explorer just executed through the
 * production middleware chain under the CURRENT failure injection.
 *
 * Reload (or flip the scenario in the header panel and reload) to
 * watch the same endpoint answer differently: the recovered 500 under
 * error injection, the slept-out 200 under latency, the instant
 * healthy row with nothing armed.
 */

type ExplorerStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ErrorsExplorerDemoData }
  | { state: 'error'; failure: Failure };

export function useErrorsExplorerDemo(
  useCase: GetErrorsExplorerDemoUseCase = container.resolve(ERRORS_EXPLORER_DEMO_USE_CASE),
): { status: ExplorerStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ExplorerStatus>({ state: 'idle' });

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

export function ErrorsExplorerDemo() {
  const { status, retry } = useErrorsExplorerDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="errors-explorer-loading" className="text-body-sm text-on-surface-variant">
        walking the probe through the middleware chain…
      </p>
    );
  }

  if (status.state === 'error') {
    return (
      <ErrorPanel subject="the errors explorer demo" failure={status.failure} onRetry={retry} />
    );
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="errors-explorer-ready">
      <section
        data-testid="errors-explorer-walk"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">
          the walk — just now, under injectFailure={data.walk.injectFailure}
        </p>
        <p className="font-mono text-caption text-on-surface">
          probe answered <span className="text-secondary">{String(data.walk.status)}</span> in{' '}
          {String(data.walk.elapsedMillis)} ms · request id{' '}
          <span className="text-secondary">{data.walk.requestID}</span>
        </p>
        <p className="font-mono text-caption text-on-surface-variant">{data.walk.body}</p>
        <p className="text-caption text-on-surface">{data.walk.note}</p>
      </section>

      {data.kinds.map((kind) => (
        <KindRow key={kind.kind} row={kind} />
      ))}

      <section
        data-testid="errors-explorer-scenario"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          <span className="text-secondary">{data.scenario.patchEndpoint}</span> — modes{' '}
          {data.scenario.modes.join(' · ')}
        </p>
        <p className="text-caption text-on-surface">{data.scenario.note}</p>
      </section>

      <section
        data-testid="errors-explorer-probe"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          <span className="text-secondary">{data.probe.probeEndpoint}</span> · latency injection{' '}
          {String(data.probe.latencyMillis)} ms
        </p>
        <p className="text-caption text-on-surface">{data.probe.healthyNote}</p>
        <p className="text-caption text-on-surface">{data.probe.injectedNote}</p>
        <p className="text-caption text-on-surface">{data.probe.latencyNote}</p>
      </section>

      <p data-testid="errors-explorer-envelope" className="text-body-sm text-on-surface-variant">
        {data.envelopeNote}
      </p>

      <p data-testid="errors-explorer-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

function KindRow({ row }: { row: ErrorsKindRow }) {
  return (
    <section
      data-testid={`errors-explorer-kind-${row.kind}`}
      className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
    >
      <p className="font-mono text-caption text-on-surface">
        <span className="text-secondary">{row.kind}</span> → {String(row.httpStatus)} ·{' '}
        {row.defaultMessage}
      </p>
      <p className="text-caption text-on-surface">{row.raisedBy}</p>
      <p className="font-mono text-caption text-on-surface-variant">{row.loggedAs}</p>
      <p className="text-caption text-on-surface">
        <span className="font-mono">{row.trigger.mode === 'live' ? 'try it' : 'recipe'}</span> —{' '}
        {row.trigger.method} {row.trigger.path}
        {row.trigger.prep ? ` (${row.trigger.prep})` : ''}
      </p>
      <p className="text-caption text-on-surface-variant">{row.trigger.note}</p>
    </section>
  );
}

export default ErrorsExplorerDemo;
