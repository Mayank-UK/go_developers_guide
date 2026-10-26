import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetRoutingExplorerDemoUseCase,
  ROUTING_EXPLORER_DEMO_USE_CASE,
} from '../../../domain/demo/get-routing-explorer-demo-usecase';
import type {
  RoutingExplorerDemoData,
  RoutingPattern,
  RoutingProbe,
} from '../../../domain/demo/routing-explorer';

/**
 * RoutingExplorerDemo — the routing topic's live demo (demo block id
 * `routing:explorer`). It renders the payload of
 * GET /api/demo/routing/explorer: the guide's REAL route table carried as
 * data (method, pattern, shape, and the handler constructor each line
 * wires), nine (method, path) probes resolved by the server's
 * ServeMux-faithful mini-matcher — matched patterns with their captured
 * path values and parsed query parameters, 405s with the mux-computed
 * Allow (HEAD included), 404s, and the escaped-path capture — plus two
 * labeled exhibits (literal-beats-wildcard precedence; the subtree 307)
 * and the verified semantics facts. Every verdict is pinned against a
 * real ServeMux by the server's tests.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load.
 */

type RoutingExplorerStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: RoutingExplorerDemoData }
  | { state: 'error'; failure: Failure };

export function useRoutingExplorerDemo(
  useCase: GetRoutingExplorerDemoUseCase = container.resolve(ROUTING_EXPLORER_DEMO_USE_CASE),
): { status: RoutingExplorerStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<RoutingExplorerStatus>({ state: 'idle' });

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

export function RoutingExplorerDemo() {
  const { status, retry } = useRoutingExplorerDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="routing-explorer-loading" className="text-body-sm text-on-surface-variant">
        resolving the route table…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the routing explorer demo" failure={status.failure} onRetry={retry} />;
  }

  const { routes, probes, exhibits, semantics } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section data-testid="re-routes" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          The route table, as the server wires it
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">method</th>
                <th scope="col" className="px-sm py-xxs">pattern</th>
                <th scope="col" className="px-sm py-xxs">shape</th>
                <th scope="col" className="px-sm py-xxs">handler</th>
              </tr>
            </thead>
            <tbody>
              {routes.map((route) => (
                <PatternCells key={`${route.method} ${route.pattern}`} route={route} />
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          {routes.length} patterns — the mux exposes no way to enumerate a registered table, so
          the demo carries it as data beside the mux.
        </p>
      </section>

      <section data-testid="re-probes" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Nine probes, four verdicts
        </h4>
        <ul className="flex flex-col gap-sm">
          {probes.map((probe) => (
            <ProbeRow key={`${probe.method} ${probe.path}`} probe={probe} />
          ))}
        </ul>
      </section>

      {exhibits.map((exhibit) => (
        <section key={exhibit.id} data-testid={`re-exhibit-${exhibit.id}`} className="flex flex-col gap-xs">
          <h4 className="font-display text-title-sm text-on-surface">{exhibit.title}</h4>
          <p className="text-body-sm text-on-surface-variant">{exhibit.note}</p>
          <div className="flex flex-wrap gap-xxs">
            {exhibit.patterns.map((pattern) => (
              <span
                key={pattern.pattern}
                className="rounded border border-outline px-xs py-xxs font-mono text-caption text-on-surface"
              >
                {pattern.method} {pattern.pattern}
                {pattern.handler ? '' : ' (exhibit)'}
              </span>
            ))}
          </div>
          <ul className="flex flex-col gap-sm">
            {exhibit.probes.map((probe) => (
              <ProbeRow key={`${probe.method} ${probe.path}`} probe={probe} />
            ))}
          </ul>
        </section>
      ))}

      <section data-testid="re-semantics" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The verified rules, as facts
        </h4>
        <div className="flex flex-wrap gap-xxs">
          <SemanticsChip fact={semantics.getAlsoServesHead ? 'GET also serves HEAD' : 'GET does not serve HEAD'} />
          <SemanticsChip
            fact={semantics.methodlessMatchesAll ? 'method-less pattern matches every method' : 'method-less pattern is restricted'}
          />
          <SemanticsChip fact={`subtree redirect: ${semantics.subtreeRedirectStatus}`} />
          <SemanticsChip fact={semantics.reverseSlashRedirect ? 'reverse-slash redirect exists' : 'no reverse-slash redirect'} />
          <SemanticsChip fact={semantics.matchesEscapedPath ? 'matches on the escaped path' : 'matches on the decoded path'} />
          <SemanticsChip fact={semantics.pathValueIsUnescaped ? 'PathValue is unescaped' : 'PathValue is escaped'} />
        </div>
        <p className="text-body-sm text-on-surface-variant">
          Every rule is agreed probe-by-probe with a real ServeMux in the server's tests — the
          demo never recites from memory.
        </p>
      </section>
    </div>
  );
}

function PatternCells({ route }: { route: RoutingPattern }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-secondary">{route.method}</td>
      <td className="px-sm py-xxs text-on-surface">{route.pattern}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{route.kind}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{route.handler ?? '—'}</td>
    </tr>
  );
}

function ProbeRow({ probe }: { probe: RoutingProbe }) {
  const failing = probe.status >= 400;
  return (
    <li className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm">
      <p className="font-mono text-body-sm text-on-surface">
        <span className="text-secondary">{probe.method}</span> {probe.path}{' '}
        <span className={failing ? 'text-error' : 'text-secondary'}>→ {probe.status}</span>{' '}
        <span className="text-on-surface-variant">{probe.outcome}</span>
      </p>
      {probe.matchedPattern ? (
        <p className="font-mono text-caption text-on-surface-variant">matched: {probe.matchedPattern}</p>
      ) : null}
      {probe.pathValues && probe.pathValues.length > 0 ? (
        <p className="font-mono text-caption text-on-surface-variant">
          path: {probe.pathValues.map((value) => `${value.name}=${value.values.join(',')}`).join(' ')}
        </p>
      ) : null}
      {probe.queryValues && probe.queryValues.length > 0 ? (
        <p className="font-mono text-caption text-on-surface-variant">
          query: {probe.queryValues.map((value) => `${value.name}=[${value.values.join(', ')}]`).join(' ')}
        </p>
      ) : null}
      {probe.allow && probe.allow.length > 0 ? (
        <p className="font-mono text-caption text-on-surface-variant">allow: {probe.allow.join(', ')}</p>
      ) : null}
      {probe.redirectTarget ? (
        <p className="font-mono text-caption text-on-surface-variant">location: {probe.redirectTarget}</p>
      ) : null}
      <p className="text-body-sm text-on-surface">{probe.reasoning}</p>
    </li>
  );
}

function SemanticsChip({ fact }: { fact: string }) {
  return (
    <span className="rounded border border-outline px-xs py-xxs font-mono text-caption text-on-surface-variant">
      {fact}
    </span>
  );
}

export default RoutingExplorerDemo;
