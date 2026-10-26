import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  ExecuteGraphQLDemoQueryUseCase,
  API_GRAPHQL_DEMO_QUERY_USE_CASE,
} from '../../../domain/demo/execute-graphql-demo-query-usecase';
import {
  GetGraphQLPlaygroundDemoUseCase,
  API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE,
} from '../../../domain/demo/get-graphql-playground-demo-usecase';
import type {
  GraphQLErrorView,
  GraphQLPlaygroundDemoData,
  GraphQLRunResult,
} from '../../../domain/demo/graphql-playground';

/**
 * GraphQLPlaygroundDemo — the graphql topic's live demo (demo block id
 * `api:graphql-playground`). A playground-style console over the posts
 * graph:
 *
 * 1. THE CARD — GET /api/demo/api/graphql-playground: the endpoint's
 *    transport contract, the schema card (root signatures + the domain
 *    types) read off the real parsed schema, and the N+1-versus-loader
 *    experiment's port-call counts.
 * 2. THE CONSOLE — the payload's executed sample runs (in-process, the
 *    deterministic reference) beside a selectable sample list; RUN posts
 *    the selected document to the LIVE POST /api/graphql and renders the
 *    response beside the reference — errors included, extensions codes
 *    and all.
 *
 * Dependency flow mirrors the views: hooks resolve use cases through the
 * container (default parameters are the spec seam), expected failures
 * arrive as typed Failures inside Results, and GraphQL errors riding a
 * 200 are DATA the console renders, not failures it hides.
 */

type CardStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: GraphQLPlaygroundDemoData }
  | { state: 'error'; failure: Failure };

type RunStatus =
  | { state: 'idle' }
  | { state: 'running' }
  | { state: 'done'; query: string; data: Record<string, unknown> | null; errors: GraphQLErrorView[] }
  | { state: 'error'; failure: Failure };

export function useGraphQLPlaygroundCard(
  useCase: GetGraphQLPlaygroundDemoUseCase = container.resolve(API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE),
): { status: CardStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<CardStatus>({ state: 'idle' });

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

  return { status, retry: () => setAttempt((current) => current + 1) };
}

export function useGraphQLRunner(
  runUseCase: ExecuteGraphQLDemoQueryUseCase = container.resolve(API_GRAPHQL_DEMO_QUERY_USE_CASE),
): { status: RunStatus; run: (query: string) => void } {
  const [status, setStatus] = useState<RunStatus>({ state: 'idle' });

  const run = (query: string) => {
    setStatus({ state: 'running' });
    void runUseCase.execute(query).then((result) => {
      if (result.ok) {
        setStatus({
          state: 'done',
          query,
          data: result.value.data,
          errors: result.value.errors,
        });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });
  };

  return { status, run };
}

export function GraphQLPlaygroundDemo() {
  const { status: card, retry } = useGraphQLPlaygroundCard();
  const { status: runStatus, run } = useGraphQLRunner();
  const [selected, setSelected] = useState(0);

  if (card.state === 'idle' || card.state === 'loading') {
    return (
      <p data-testid="gqlp-loading" className="text-body-sm text-on-surface-variant">
        executing the sample operations…
      </p>
    );
  }

  if (card.state === 'error') {
    return <ErrorPanel subject="the graphql playground" failure={card.failure} onRetry={retry} />;
  }

  const { endpoint, schema, samples, runs, loader } = card.data;
  const safeSelected = Math.min(selected, samples.length - 1);
  const sample = samples[safeSelected];
  if (sample === undefined) {
    return (
      <p data-testid="gqlp-empty" className="text-body-sm text-on-surface-variant">
        the payload carried no samples — nothing to select or run
      </p>
    );
  }
  const reference = runs.find((candidate) => candidate.id === sample.id);

  return (
    <div className="flex flex-col gap-md">
      <EndpointCard endpoint={endpoint} />

      <section data-testid="gqlp-schema" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">The schema card — read off the real parsed schema</h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">operation</th>
                <th scope="col" className="px-sm py-xxs">type</th>
                <th scope="col" className="px-sm py-xxs">fields</th>
              </tr>
            </thead>
            <tbody>
              {schema.operations.map((signature) => (
                <tr key={signature} className="border-t border-outline">
                  <td colSpan={3} className="px-sm py-xxs text-secondary">{signature}</td>
                </tr>
              ))}
              {schema.types.map((type) => (
                <tr key={type.name} className="border-t border-outline align-top">
                  <td colSpan={2} className="px-sm py-xxs text-on-surface">{type.name}</td>
                  <td className="px-sm py-xxs text-on-surface-variant">
                    <span className="text-secondary">{type.kind}</span>{' '}
                    {type.fields.map((field) => `${field.name}: ${field.type}`).join(' · ')}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="gqlp-console" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">The console — pick an operation, run it live</h4>
        <div className="flex flex-wrap gap-xxs" data-testid="gqlp-samples">
          {samples.map((candidate, index) => (
            <button
              key={candidate.id}
              type="button"
              data-testid={`gqlp-sample-${candidate.id}`}
              onClick={() => setSelected(index)}
              className={`rounded border px-sm py-xxs font-mono text-caption transition-colors ${
                index === safeSelected
                  ? 'border-secondary bg-surface-tint text-on-surface'
                  : 'border-outline text-on-surface-variant hover:text-on-surface'
              }`}
            >
              {candidate.kind} · {candidate.label}
            </button>
          ))}
        </div>

        <div className="grid grid-cols-1 gap-sm md:grid-cols-2">
          <div className="flex flex-col gap-xxs">
            <p className="font-mono text-caption uppercase tracking-wide text-on-surface-variant">
              the document — POST {endpoint.path}
            </p>
            <pre className="overflow-x-auto rounded-md border border-outline bg-surface-tint px-sm py-xxs font-mono text-caption text-on-surface">
              {sample.query}
            </pre>
            <p className="text-body-sm text-on-surface-variant">{sample.note}</p>
            <button
              type="button"
              data-testid="gqlp-run-button"
              onClick={() => run(sample.query)}
              disabled={runStatus.state === 'running'}
              className="self-start rounded border border-secondary px-md py-xxs text-body-sm text-secondary transition-colors hover:bg-surface-tint disabled:opacity-50"
            >
              {runStatus.state === 'running' ? 'running…' : 'RUN against the live endpoint'}
            </button>
            <RunResult status={runStatus} />
          </div>

          <div className="flex flex-col gap-xxs">
            <p className="font-mono text-caption uppercase tracking-wide text-on-surface-variant">
              the reference run — executed in-process by the server
            </p>
            {reference ? <RunDocument run={reference} /> : null}
          </div>
        </div>
      </section>

      <section data-testid="gqlp-loader" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">The N+1 experiment, as numbers</h4>
        <div className="grid grid-cols-2 gap-xs md:grid-cols-4">
          <CounterCard label="naive port calls" value={loader.naivePortCalls} hint="1 list + 6 getById" />
          <CounterCard label="loader port calls" value={loader.loaderPortCalls} hint="one batched call" />
          <CounterCard label="loads issued" value={loader.loadsIssued} hint={`${loader.duplicateLoads} duplicates`} />
          <CounterCard label="all resolved" value={loader.allResolved ? 'yes' : 'no'} hint={`${loader.uniqueIds} unique ids`} />
        </div>
        <p className="text-body-sm text-on-surface-variant">{loader.verdict}</p>
      </section>
    </div>
  );
}

function EndpointCard({ endpoint }: { endpoint: GraphQLPlaygroundDemoData['endpoint'] }) {
  return (
    <section
      data-testid="gqlp-endpoint"
      className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
    >
      <h4 className="font-display text-title-sm text-on-surface">One endpoint, one document grammar</h4>
      <p className="font-mono text-body-sm text-on-surface">
        {endpoint.method} {endpoint.path}
      </p>
      <pre className="overflow-x-auto font-mono text-caption text-secondary">{endpoint.requestBody}</pre>
      <p className="text-body-sm text-on-surface-variant">{endpoint.liveNote}</p>
    </section>
  );
}

function RunResult({ status }: { status: RunStatus }) {
  if (status.state === 'idle') {
    return (
      <p data-testid="gqlp-run-idle" className="text-body-sm text-on-surface-variant">
        RUN posts this document to the live endpoint — the answer lands here, errors and all.
      </p>
    );
  }
  if (status.state === 'running') {
    return (
      <p data-testid="gqlp-run-running" className="text-body-sm text-on-surface-variant">
        posting to {`{`}apiBaseUrl{`}`}/api/graphql…
      </p>
    );
  }
  if (status.state === 'error') {
    return <ErrorPanel subject="the live run" failure={status.failure} onRetry={() => undefined} />;
  }
  return (
    <div data-testid="gqlp-run-result" className="flex flex-col gap-xxs">
      <p className="font-mono text-caption uppercase tracking-wide text-on-surface-variant">
        the live answer — errors are data, not failures
      </p>
      <DocumentBlock data={status.data} />
      {status.errors.length > 0 ? <ErrorViews errors={status.errors} /> : null}
    </div>
  );
}

function RunDocument({ run }: { run: GraphQLRunResult }) {
  return (
    <div className="flex flex-col gap-xxs">
      <DocumentBlock data={run.data} />
      {run.errors.length > 0 ? <ErrorViews errors={run.errors} /> : null}
      {run.notes.map((note) => (
        <p key={note} className="text-caption text-on-surface-variant">{note}</p>
      ))}
    </div>
  );
}

function DocumentBlock({ data }: { data: Record<string, unknown> | null }) {
  return (
    <pre className="max-h-64 overflow-auto rounded-md border border-outline bg-surface-tint px-sm py-xxs font-mono text-caption text-on-surface">
      {JSON.stringify(data, null, 2)}
    </pre>
  );
}

function ErrorViews({ errors }: { errors: GraphQLErrorView[] }) {
  return (
    <ul data-testid="gqlp-errors" className="flex flex-col gap-xxs">
      {errors.map((error, index) => (
        <li key={index} className="rounded border border-outline px-sm py-xxs">
          <p className="text-body-sm text-error">{error.message}</p>
          {error.extensions ? (
            <p className="font-mono text-caption text-on-surface-variant">
              extensions {JSON.stringify(error.extensions)}
            </p>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

function CounterCard({ label, value, hint }: { label: string; value: number | string; hint: string }) {
  return (
    <div className="rounded-md border border-outline px-sm py-xxs">
      <p className="font-display text-title-md text-secondary">{value}</p>
      <p className="font-mono text-caption uppercase tracking-wide text-on-surface-variant">{label}</p>
      <p className="text-caption text-on-surface-variant">{hint}</p>
    </div>
  );
}

export default GraphQLPlaygroundDemo;
