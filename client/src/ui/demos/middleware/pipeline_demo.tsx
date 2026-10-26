import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetMiddlewarePipelineDemoUseCase,
  MIDDLEWARE_PIPELINE_DEMO_USE_CASE,
} from '../../../domain/demo/get-middleware-pipeline-demo-usecase';
import type {
  ChainLayer,
  MiddlewarePipelineDemoData,
  TraceEvent,
} from '../../../domain/demo/middleware-pipeline';

/**
 * PipelineDemo — the middleware topic's live demo (demo block id
 * `middleware:pipeline`). It renders the payload of
 * GET /api/demo/middleware/pipeline: the exhibit chain outermost-first
 * (each row naming the production layer it mirrors), the traced run as a
 * numbered table — before-beats descending, the handler once in the
 * middle, after-beats unwinding in reverse — the request id's
 * propagation onto the response header, the recovered-panic beat (the
 * deliberate bug, the 500 envelope, the captured log line), and the
 * http.TimeoutHandler beat (budget vs sleep vs observed 503, abandoned
 * goroutine joined). The first trace beat's empty `requestIDSeen` is
 * rendered as data: the id does not exist yet — assigning it is that
 * layer's job.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load.
 */

type PipelineDemoStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: MiddlewarePipelineDemoData }
  | { state: 'error'; failure: Failure };

export function usePipelineDemo(
  useCase: GetMiddlewarePipelineDemoUseCase = container.resolve(MIDDLEWARE_PIPELINE_DEMO_USE_CASE),
): { status: PipelineDemoStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<PipelineDemoStatus>({ state: 'idle' });

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

export function PipelineDemo() {
  const { status, retry } = usePipelineDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="middleware-pipeline-loading" className="text-body-sm text-on-surface-variant">
        running the traced pipeline…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the pipeline demo" failure={status.failure} onRetry={retry} />;
  }

  const {
    chain,
    trace,
    requestSideOrder,
    responseSideOrder,
    requestID,
    panicRecovery,
    timeoutBeat,
    reasoning,
  } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section data-testid="mp-chain" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">The chain, outermost first</h4>
        <ol className="flex flex-col gap-xxs">
          {chain.map((layer) => (
            <ChainRow key={layer.name} layer={layer} />
          ))}
        </ol>
        <p className="font-mono text-caption text-on-surface-variant">
          request: {requestSideOrder.join(' → ')}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          response: {responseSideOrder.join(' → ')}
        </p>
      </section>

      <section data-testid="mp-trace" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">The onion, as one request lived it</h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">#</th>
                <th scope="col" className="px-sm py-xxs">layer</th>
                <th scope="col" className="px-sm py-xxs">phase</th>
                <th scope="col" className="px-sm py-xxs">id seen</th>
                <th scope="col" className="px-sm py-xxs">note</th>
              </tr>
            </thead>
            <tbody>
              {trace.map((event) => (
                <TraceCells key={event.sequence} event={event} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section
        data-testid="mp-request-id"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">One id, everywhere</h4>
        <p className="font-mono text-body-sm text-on-surface">
          {requestID.headerName}: {requestID.inbound} → echoed as {requestID.responseHeader}
        </p>
        <p className="text-body-sm text-on-surface-variant">
          {requestID.sameIDEverywhere
            ? 'Every layer past the first observed the same id — and the response header echoes it.'
            : 'The id did not survive every layer — investigate before trusting this run.'}
        </p>
      </section>

      <section data-testid="mp-panic" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The deliberate panic, converted
        </h4>
        <p className="text-body-sm text-on-surface-variant">
          The terminal handler panicked (<span className="font-mono text-error">{panicRecovery.cause}</span>);
          the recover layer answered{' '}
          <span className="font-mono text-on-surface">{panicRecovery.status}</span> with the shared
          envelope{panicRecovery.requestIDKept ? ' — request id intact' : ''}.
        </p>
        <pre className="overflow-x-auto rounded-md border border-outline bg-surface-tint px-sm py-xs font-mono text-caption text-on-surface">
          {panicRecovery.loggedLine}
        </pre>
      </section>

      <section data-testid="mp-timeout" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The timeout beat: {timeoutBeat.wrapper}
        </h4>
        <p className="font-mono text-body-sm text-on-surface">
          budget {timeoutBeat.budgetMs}ms vs sleep {timeoutBeat.handlerSleepMs}ms →{' '}
          {timeoutBeat.observedStatus}
        </p>
        <p className="text-body-sm text-on-surface-variant">
          {timeoutBeat.slowHandlerFinished
            ? 'The abandoned handler goroutine finished within a bounded join — no leak.'
            : 'The abandoned handler goroutine did not report completion.'}{' '}
          {timeoutBeat.shape}
        </p>
      </section>

      <p data-testid="mp-reasoning" className="text-body-sm text-on-surface-variant">
        {reasoning}
      </p>
    </div>
  );
}

function ChainRow({ layer }: { layer: ChainLayer }) {
  return (
    <li className="flex flex-col gap-xxs rounded-md border border-outline px-sm py-xxs">
      <span className="font-mono text-caption text-on-surface">
        {layer.position}. {layer.name}
      </span>
      <span className="text-body-sm text-on-surface-variant">{layer.role}</span>
      <span className="font-mono text-caption text-secondary">mirrors {layer.mirrors}</span>
    </li>
  );
}

function TraceCells({ event }: { event: TraceEvent }) {
  const unassigned = event.requestIDSeen === '';
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface-variant">{event.sequence}</td>
      <td className="px-sm py-xxs text-on-surface">{event.layer}</td>
      <td
        className={`px-sm py-xxs ${event.phase === 'handler' ? 'text-secondary' : 'text-on-surface-variant'}`}
      >
        {event.phase}
      </td>
      <td className={`px-sm py-xxs font-mono ${unassigned ? 'text-on-surface-variant' : 'text-secondary'}`}>
        {unassigned ? '— not yet assigned' : event.requestIDSeen}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">{event.note}</td>
    </tr>
  );
}

export default PipelineDemo;
