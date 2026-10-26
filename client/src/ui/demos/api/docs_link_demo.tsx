import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetDocsLinkDemoUseCase,
  API_DOCS_LINK_DEMO_USE_CASE,
} from '../../../domain/demo/get-docs-link-demo-usecase';
import type { DocsLinkDemoData } from '../../../domain/demo/docs-link';

/**
 * DocsLinkDemo — the api-documentation topic's live demo (demo block id
 * `api:docs-link`). It renders the payload of GET /api/demo/api/docs-link:
 * the OpenAPI contract as DATA. Every fact — the spec's own title,
 * version, and servers, the path/operation/schema counts, the operations
 * table, the schema inventory, the drift guarantees — was parsed
 * SERVER-SIDE from the actual server/api/openapi.yaml at request time, so
 * the card cannot drift from the document; the raw-spec pane shows the
 * very bytes /api/openapi.yaml serves, linked beside the story of how
 * Swagger UI and friends consume that same URL.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load.
 */

type DocsLinkStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: DocsLinkDemoData }
  | { state: 'error'; failure: Failure };

export function useDocsLinkDemo(
  useCase: GetDocsLinkDemoUseCase = container.resolve(API_DOCS_LINK_DEMO_USE_CASE),
): { status: DocsLinkStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<DocsLinkStatus>({ state: 'idle' });

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

export function DocsLinkDemo() {
  const { status, retry } = useDocsLinkDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="docs-link-loading" className="text-body-sm text-on-surface-variant">
        parsing the spec…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the docs-link demo" failure={status.failure} onRetry={retry} />;
  }

  const { spec, operations, schemas, serving, drift, specText } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section data-testid="dl-facts" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The spec's own facts — parsed from the file, not copied
        </h4>
        <dl className="grid grid-cols-1 gap-xs md:grid-cols-3">
          <Fact label="title" value={spec.title} />
          <Fact label="version" value={`${spec.version} · OpenAPI ${spec.openapiVersion}`} />
          <Fact label="servers" value={spec.servers.join(' · ')} />
          <Fact label="paths" value={`${spec.pathCount}`} />
          <Fact label="operations" value={`${spec.operationCount}`} />
          <Fact label="schemas" value={`${spec.schemaCount}`} />
        </dl>
        <p className="text-body-sm text-on-surface-variant">
          <span className="font-mono text-on-surface">{spec.specPath}</span> is the source of truth;{' '}
          {spec.validationIssue}.
        </p>
      </section>

      <section data-testid="dl-operations" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">The operation inventory</h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">method</th>
                <th scope="col" className="px-sm py-xxs">path</th>
                <th scope="col" className="px-sm py-xxs">summary</th>
              </tr>
            </thead>
            <tbody>
              {operations.map((operation) => (
                <tr key={`${operation.method}-${operation.path}-${operation.id}`} className="border-t border-outline">
                  <td className="px-sm py-xxs text-secondary">{operation.method}</td>
                  <td className="px-sm py-xxs text-on-surface">{operation.path}</td>
                  <td className="px-sm py-xxs font-sans whitespace-normal text-on-surface-variant">{operation.summary}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="flex flex-wrap gap-xxs">
          {schemas.map((schema) => (
            <span
              key={schema.name}
              title={schema.description}
              className="rounded border border-outline px-xs py-xxs font-mono text-caption text-on-surface"
            >
              {schema.name}
            </span>
          ))}
        </div>
        <p className="text-body-sm text-on-surface-variant">
          The schema chips are what oapi-codegen turned into committed Go types.
        </p>
      </section>

      <section data-testid="dl-drift" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The drift guarantees — what the test enforces
        </h4>
        <ul className="flex flex-col gap-xxs">
          {drift.map((row) => (
            <li key={row.check} className="text-body-sm text-on-surface-variant">
              <span className="font-mono text-on-surface">{row.check}</span> — {row.compares}; {row.settled}.
            </li>
          ))}
        </ul>
      </section>

      <section data-testid="dl-serving" className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">The document, served</h4>
        <p className="text-body-sm text-on-surface">
          <a
            href={serving.specURL}
            className="font-mono text-secondary underline decoration-dotted underline-offset-4"
            target="_blank"
            rel="noreferrer"
          >
            {serving.specURL}
          </a>{' '}
          — served verbatim as <span className="font-mono">{serving.contentType}</span>
          {serving.servedVerbatim ? ', byte for byte from the embedded file' : ''}. {serving.swaggerStory}.
        </p>
      </section>

      <section data-testid="dl-raw" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">The raw spec — the bytes the route serves</h4>
        <pre
          className="max-h-80 overflow-auto rounded-md border border-outline bg-surface-tint px-sm py-xs font-mono text-caption leading-relaxed text-on-surface"
        >
          {specText}
        </pre>
      </section>
    </div>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-outline bg-surface-tint px-sm py-xxs">
      <dt className="font-mono text-caption uppercase tracking-wide text-on-surface-variant">{label}</dt>
      <dd className="text-body-sm text-on-surface">{value}</dd>
    </div>
  );
}

export default DocsLinkDemo;
