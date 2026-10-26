import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetHttpHeadersDemoUseCase,
  HTTP_HEADERS_DEMO_USE_CASE,
} from '../../../domain/demo/get-http-headers-demo-usecase';
import {
  REDACTED_HEADER_VALUE,
  type EchoedHeader,
  type HttpHeadersDemoData,
} from '../../../domain/demo/http-headers';

/**
 * HeadersDemo — the http-web topic's live demo (demo block id
 * `http:headers`). It renders the payload of GET /api/demo/http/headers:
 * the server's echo of THIS VERY REQUEST — the request line it routed
 * (method, path, raw query, negotiated protocol), the request id the
 * middleware honored or minted, and every inbound header classified twice:
 * scope (end-to-end vs hop-by-hop, RFC 9110 §7.6.1) and role (negotiation,
 * CORS, security, identity, extension, transport). Credential bearers are
 * detected with their values withheld — an echo must never become a leak —
 * and the closed hop-by-hop reference list rides along as chips.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load (a fresh echo:
 * the retried fetch describes the retry).
 */

type HeadersDemoStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: HttpHeadersDemoData }
  | { state: 'error'; failure: Failure };

export function useHeadersDemo(
  useCase: GetHttpHeadersDemoUseCase = container.resolve(HTTP_HEADERS_DEMO_USE_CASE),
): { status: HeadersDemoStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<HeadersDemoStatus>({ state: 'idle' });

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

export function HeadersDemo() {
  const { status, retry } = useHeadersDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="http-headers-loading" className="text-body-sm text-on-surface-variant">
        echoing your request…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the headers demo" failure={status.failure} onRetry={retry} />;
  }

  const { method, path, query, proto, requestID, headers, hopByHop } = status.data;
  const noted = headers.filter((header) => header.note !== undefined);

  return (
    <div className="flex flex-col gap-md">
      <section
        data-testid="hh-request-line"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">The request line, as routed</h4>
        <p className="font-mono text-body-sm text-on-surface">
          {method} {path}
          {query ? <span className="text-on-surface-variant">?{query}</span> : null}{' '}
          <span className="text-secondary">{proto}</span>
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          x-request-id: {requestID} — echoed by the middleware, minted when you did not send one
        </p>
      </section>

      <section data-testid="hh-table" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Every header you sent, classified
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">header</th>
                <th scope="col" className="px-sm py-xxs">value</th>
                <th scope="col" className="px-sm py-xxs">scope</th>
                <th scope="col" className="px-sm py-xxs">role</th>
              </tr>
            </thead>
            <tbody>
              {headers.map((header) => (
                <HeaderCells key={header.name} header={header} />
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          Credentials report their presence with the value withheld — an echo
          that leaks bearer tokens would be its own counter-example.
        </p>
      </section>

      {noted.length > 0 ? (
        <section data-testid="hh-notes" className="flex flex-col gap-xs">
          <h4 className="font-display text-title-sm text-on-surface">Notes from the server</h4>
          <ul className="flex flex-col gap-xxs">
            {noted.map((header) => (
              <li key={header.name} className="text-body-sm text-on-surface-variant">
                <span className="font-mono text-on-surface">{header.name}</span> — {header.note}
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <section data-testid="hh-hop-by-hop" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The hop-by-hop closed set (RFC 9110 §7.6.1 + keep-alive)
        </h4>
        <div className="flex flex-wrap gap-xxs">
          {hopByHop.map((name) => (
            <span
              key={name}
              className="rounded border border-outline px-xs py-xxs font-mono text-caption text-on-surface-variant"
            >
              {name}
            </span>
          ))}
        </div>
        <p className="text-body-sm text-on-surface-variant">
          These describe one transport connection — consumed per hop, never
          forwarded. Send one and watch it land in the table above with
          scope <span className="font-mono">hop-by-hop</span>.
        </p>
      </section>
    </div>
  );
}

function HeaderCells({ header }: { header: EchoedHeader }) {
  const withheld = header.value === REDACTED_HEADER_VALUE;
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{header.name}</td>
      <td className={`px-sm py-xxs ${withheld ? 'text-error' : 'text-secondary'}`}>{header.value}</td>
      <td
        className={`px-sm py-xxs ${header.scope === 'hop-by-hop' ? 'text-secondary' : 'text-on-surface-variant'}`}
      >
        {header.scope}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">{header.role}</td>
    </tr>
  );
}

export default HeadersDemo;
