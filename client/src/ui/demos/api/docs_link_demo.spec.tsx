import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { DocsLinkDemo } from './docs_link_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetDocsLinkDemoUseCase,
  API_DOCS_LINK_DEMO_USE_CASE,
} from '../../../domain/demo/get-docs-link-demo-usecase';
import type { DocsLinkDemoData } from '../../../domain/demo/docs-link';

/**
 * DocsLinkDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration instead
 * of stubbing fetch. Behavior, not implementation: the spec facts render
 * from the payload, the operations table lists the five real routes, the
 * schema chips and drift rows render, the serving story links the spec
 * URL, the raw spec pane shows the verbatim text, and failures degrade
 * to the shared error panel with recovery.
 *
 * (The demo-registry binding test lands with the composition-root wiring —
 * this slice ships before demo-registry.tsx grows its lazy entry.)
 */

const fixture: DocsLinkDemoData = {
  spec: {
    title: 'Go Developers Guide — posts API',
    version: '1.0.0',
    openapiVersion: '3.1.0',
    specPath: 'server/api/openapi.yaml',
    specURL: '/api/openapi.yaml',
    servers: ['http://localhost:8100'],
    pathCount: 2,
    operationCount: 5,
    schemaCount: 6,
    validationIssue: 'the drift test walks live JSON against these schemas',
  },
  operations: [
    { id: 'listPosts', method: 'GET', path: '/api/posts', summary: 'One page of the posts collection' },
    { id: 'createPost', method: 'POST', path: '/api/posts', summary: 'Create a post from a full draft body' },
    { id: 'getPost', method: 'GET', path: '/api/posts/{id}', summary: 'Read one post by id' },
    { id: 'replacePost', method: 'PUT', path: '/api/posts/{id}', summary: 'Full replacement — the body is the whole next state' },
    { id: 'deletePost', method: 'DELETE', path: '/api/posts/{id}', summary: 'Remove a post under the strict stance' },
  ],
  schemas: [
    { name: 'Post', description: 'One post on the wire — the response shape of read, create, and replace.' },
    { name: 'PostPage', description: 'The list envelope — one page’s items plus the honest pagination metadata.' },
    { name: 'ErrorEnvelope', description: 'Every failure on every route wears this one shape.' },
  ],
  serving: {
    specURL: '/api/openapi.yaml',
    contentType: 'application/yaml',
    servedVerbatim: true,
    swaggerStory: 'point Swagger UI at the URL and it renders the docs from the same bytes',
  },
  drift: [
    {
      check: 'list response ↔ PostPage',
      compares: 'GET /api/posts body: every key, type, and bound against the schema',
      settled: 'spec_drift_test.go drives the live handler and walks the JSON',
    },
    {
      check: 'single post ↔ Post',
      compares: 'GET /api/posts/{id} body: field names, tags never null, RFC 3339 timestamps',
      settled: 'the walker refuses unknown keys (additionalProperties: false)',
    },
  ],
  specText: 'openapi: 3.1.0\ninfo:\n  title: Go Developers Guide — posts API\n  version: 1.0.0\n',
};

function stubUseCase(execute: () => Promise<Result<DocsLinkDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    API_DOCS_LINK_DEMO_USE_CASE,
    { execute } as unknown as GetDocsLinkDemoUseCase,
  );
}

describe('DocsLinkDemo', () => {
  it('renders loading first, then the whole card', async () => {
    stubUseCase(async () => success(fixture));

    render(<DocsLinkDemo />);

    expect(screen.getByTestId('docs-link-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('dl-facts')).toBeInTheDocument());

    // The spec's own facts, parsed server-side.
    const facts = screen.getByTestId('dl-facts');
    expect(facts).toHaveTextContent('Go Developers Guide — posts API');
    expect(facts).toHaveTextContent('1.0.0 · OpenAPI 3.1.0');
    expect(facts).toHaveTextContent('http://localhost:8100');
    expect(facts).toHaveTextContent('server/api/openapi.yaml');

    // The operation inventory: all five real routes.
    const operations = screen.getByTestId('dl-operations');
    expect(operations).toHaveTextContent('GET');
    expect(operations).toHaveTextContent('POST');
    expect(operations).toHaveTextContent('PUT');
    expect(operations).toHaveTextContent('DELETE');
    expect(operations).toHaveTextContent('/api/posts/{id}');
    expect(operations).toHaveTextContent('One page of the posts collection');

    // The schema chips and the drift guarantees.
    expect(operations).toHaveTextContent('PostPage');
    expect(screen.getByTestId('dl-drift')).toHaveTextContent('list response ↔ PostPage');
    expect(screen.getByTestId('dl-drift')).toHaveTextContent('spec_drift_test.go');

    // The serving story links the spec URL.
    const serving = screen.getByTestId('dl-serving');
    expect(serving).toHaveTextContent('/api/openapi.yaml');
    expect(serving).toHaveTextContent('application/yaml');
    expect(serving).toHaveTextContent('Swagger UI');
    expect(serving.querySelector('a')?.getAttribute('href')).toBe('/api/openapi.yaml');

    // The raw spec pane shows the verbatim document.
    expect(screen.getByTestId('dl-raw')).toHaveTextContent('openapi: 3.1.0');
  });

  it('degrades to the shared error panel on a Failed Result, with recovery', async () => {
    let calls = 0;
    stubUseCase(async () => {
      calls += 1;
      return calls === 1
        ? failed(
            new ServerFailure('The demo endpoint is down.', {
              code: 'service_unavailable',
              status: 503,
              requestId: 'req-31',
            }),
          )
        : success(fixture);
    });

    render(<DocsLinkDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the card renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('dl-facts')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
