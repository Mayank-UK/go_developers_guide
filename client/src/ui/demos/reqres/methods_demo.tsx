import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetReqresMethodsDemoUseCase,
  REQRES_METHODS_DEMO_USE_CASE,
} from '../../../domain/demo/get-reqres-methods-demo-usecase';
import type { ReqresMethodRow, ReqresMethodsDemoData } from '../../../domain/demo/reqres-methods';

/**
 * ReqresMethodsDemo — the request-response topic's response-side demo
 * (demo block id `reqres:methods`). It renders the payload of
 * GET /api/demo/reqres/methods: the multi-method exhibit's OBSERVED
 * report — one row per method (status + phrase, the selected headers
 * including Location on the 201, Set-Cookie on the GET, Allow on the
 * OPTIONS and the truthful 405, the body-shape rule, the exact bytes) —
 * plus the SSE beat: three parsed events and the flush count observed
 * through a counting wrapper, no timing values anywhere.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load.
 */

type ReqresMethodsStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ReqresMethodsDemoData }
  | { state: 'error'; failure: Failure };

export function useReqresMethodsDemo(
  useCase: GetReqresMethodsDemoUseCase = container.resolve(REQRES_METHODS_DEMO_USE_CASE),
): { status: ReqresMethodsStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ReqresMethodsStatus>({ state: 'idle' });

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

export function ReqresMethodsDemo() {
  const { status, retry } = useReqresMethodsDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="reqres-methods-loading" className="text-body-sm text-on-surface-variant">
        running the exhibit…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the methods demo" failure={status.failure} onRetry={retry} />;
  }

  const { resource, allow, methods, sse } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section
        data-testid="rm-resource"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">The exhibit resource</h4>
        <p className="font-mono text-body-sm text-on-surface">{resource}</p>
        <p className="font-mono text-caption text-on-surface-variant">Allow: {allow}</p>
      </section>

      <section data-testid="rm-table" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Eight methods, observed — not claimed
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">method</th>
                <th scope="col" className="px-sm py-xxs">status</th>
                <th scope="col" className="px-sm py-xxs">headers</th>
                <th scope="col" className="px-sm py-xxs">body</th>
              </tr>
            </thead>
            <tbody>
              {methods.map((row) => (
                <MethodCells key={row.method} row={row} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="rm-sse" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The streaming beat — {sse.events.length} events, {sse.flushes} flushes
        </h4>
        <p className="font-mono text-caption text-on-surface-variant">Content-Type: {sse.contentType}</p>
        <div className="flex flex-col gap-xxs font-mono text-caption">
          {sse.events.map((event, index) => (
            <p key={index} className="text-on-surface">
              <span className="text-secondary">event:</span> {event.name}{' '}
              <span className="text-secondary">data:</span> {event.data}
            </p>
          ))}
        </div>
        <p className="text-body-sm text-on-surface-variant">{sse.framing}</p>
      </section>
    </div>
  );
}

function MethodCells({ row }: { row: ReqresMethodRow }) {
  const statusClass =
    row.status >= 400 ? 'text-error' : row.status >= 300 ? 'text-secondary' : 'text-on-surface';
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{row.method}</td>
      <td className={`px-sm py-xxs ${statusClass}`}>
        {row.status} {row.statusText}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">
        {row.headers.map((header) => (
          <span key={header.name} className="block break-all">
            <span className="text-on-surface">{header.name}:</span> {header.value}
          </span>
        ))}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">
        {row.body === '' ? (
          <span className="italic">{row.bodyShape}</span>
        ) : (
          <>
            <span className="break-all">{row.body}</span>
            <span className="block">{row.bodyShape}</span>
          </>
        )}
      </td>
    </tr>
  );
}

export default ReqresMethodsDemo;
