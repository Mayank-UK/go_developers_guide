import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetReqresInspectorDemoUseCase,
  REQRES_INSPECTOR_DEMO_USE_CASE,
} from '../../../domain/demo/get-reqres-inspector-demo-usecase';
import type {
  ReqresDecodeExhibit,
  ReqresInspectorDemoData,
  ReqresLiveBody,
} from '../../../domain/demo/reqres-inspector';

/**
 * ReqresInspectorDemo — the request-response topic's request-side demo
 * (demo block id `reqres:inspector`). It renders the payload of
 * GET /api/demo/reqres/inspector: the live GET's own facts (method,
 * path, content type, body size against the 4096-byte cap), the decoder
 * discipline stated as rules, four exhibit bodies with deterministic
 * verdicts — the error verbatim, its concrete Go type, the offending
 * field, and the decoded (possibly partial) state — and the live beat
 * applying the same discipline to whatever body THIS request carried.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load (a fresh
 * inspection: the retried fetch describes the retry).
 */

type ReqresInspectorStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ReqresInspectorDemoData }
  | { state: 'error'; failure: Failure };

export function useReqresInspectorDemo(
  useCase: GetReqresInspectorDemoUseCase = container.resolve(REQRES_INSPECTOR_DEMO_USE_CASE),
): { status: ReqresInspectorStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ReqresInspectorStatus>({ state: 'idle' });

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

export function ReqresInspectorDemo() {
  const { status, retry } = useReqresInspectorDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="reqres-inspector-loading" className="text-body-sm text-on-surface-variant">
        inspecting your request…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the inspector demo" failure={status.failure} onRetry={retry} />;
  }

  const { request, discipline, exhibits, liveBody } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section
        data-testid="ri-request"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">This very GET, as the handler saw it</h4>
        <p className="font-mono text-body-sm text-on-surface">
          {request.method} {request.path}
          {request.contentType ? <span className="text-secondary"> ({request.contentType})</span> : null}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          body: {request.bodyBytes} / {request.bodyLimit} bytes
          {request.bodyLimitExceeded ? ' — past the cap, read truncated' : ''}
        </p>
      </section>

      <section data-testid="ri-discipline" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">The decoder discipline</h4>
        <dl className="flex flex-col gap-xxs">
          {(
            [
              ['stream', discipline.decoder],
              ['unknown fields', discipline.unknownFields],
              ['trailing', discipline.trailing],
              ['first error', discipline.firstError],
            ] as const
          ).map(([term, note]) => (
            <div key={term} className="text-body-sm">
              <dt className="inline font-mono text-on-surface">{term}:</dt>{' '}
              <dd className="inline text-on-surface-variant">{note}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section data-testid="ri-exhibits" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Four bodies, four verdicts — deterministic, error verbatim
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">exhibit</th>
                <th scope="col" className="px-sm py-xxs">verdict</th>
                <th scope="col" className="px-sm py-xxs">first error</th>
                <th scope="col" className="px-sm py-xxs">decoded</th>
              </tr>
            </thead>
            <tbody>
              {exhibits.map((exhibit) => (
                <ExhibitCells key={exhibit.label} exhibit={exhibit} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="ri-live" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">Your body, same discipline</h4>
        <LiveBodyCells live={liveBody} />
      </section>
    </div>
  );
}

function ExhibitCells({ exhibit }: { exhibit: ReqresDecodeExhibit }) {
  const accepted = exhibit.outcome === 'accepted';
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{exhibit.label}</td>
      <td className={`px-sm py-xxs ${accepted ? 'text-secondary' : 'text-error'}`}>{exhibit.outcome}</td>
      <td className="px-sm py-xxs text-on-surface-variant">
        {accepted ? (
          '—'
        ) : (
          <>
            {exhibit.error}
            <br />
            <span className="text-on-surface">{exhibit.errorType}</span>
            {exhibit.field ? ` (field: ${exhibit.field})` : ''}
            {exhibit.unexpectedEOF ? ' · errors.Is(io.ErrUnexpectedEOF)' : ''}
          </>
        )}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">{exhibit.decoded}</td>
    </tr>
  );
}

function LiveBodyCells({ live }: { live: ReqresLiveBody }) {
  if (!live.sent) {
    return (
      <p className="text-body-sm text-on-surface-variant">
        outcome: <span className="font-mono text-on-surface">none</span> — a plain GET carries no
        body; absence stated, never guessed around.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-xxs font-mono text-caption">
      <p className="text-on-surface">
        outcome: <span className={live.outcome === 'accepted' ? 'text-secondary' : 'text-error'}>{live.outcome}</span>
      </p>
      <p className="break-all text-on-surface-variant">{live.body}</p>
      {live.error ? <p className="text-error">{live.error}</p> : null}
      {live.decoded !== undefined ? <p className="text-on-surface">{live.decoded}</p> : null}
    </div>
  );
}

export default ReqresInspectorDemo;
