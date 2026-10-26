import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { GraphQLPlaygroundDemo } from './graphql_playground_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import {
  ExecuteGraphQLDemoQueryUseCase,
  API_GRAPHQL_DEMO_QUERY_USE_CASE,
} from '../../../domain/demo/execute-graphql-demo-query-usecase';
import {
  GetGraphQLPlaygroundDemoUseCase,
  API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE,
} from '../../../domain/demo/get-graphql-playground-demo-usecase';
import type { GraphQLPlaygroundDemoData } from '../../../domain/demo/graphql-playground';

/**
 * GraphQLPlaygroundDemo spec — fixture-driven through the container seams
 * (the payload use case and the RUN use case under their tokens).
 * Behavior, not implementation: the card renders the endpoint and schema
 * signatures; samples are selectable and the reference run shows beside
 * the document; RUN drives the live-query use case with the selected
 * document's exact text and renders the live answer — GraphQL errors
 * included as data, never hidden.
 */

const payload: GraphQLPlaygroundDemoData = {
  endpoint: {
    path: '/api/graphql',
    method: 'POST',
    requestBody: '{"query":"{ posts(limit: 2) { items { id title } } }"}',
    liveNote: 'the runs execute the same generated executor in-process',
  },
  schema: {
    operations: ['query posts(page: Int, limit: Int, tag: String, sort: String): PostPage!'],
    types: [
      { name: 'Post', kind: 'object', fields: [{ name: 'id', type: 'ID!' }, { name: 'title', type: 'String!' }] },
    ],
  },
  samples: [
    {
      id: 'list-first-page',
      label: 'List the first page (paginated)',
      kind: 'query',
      query: 'query { posts(page: 1, limit: 2) { items { id } } }',
      note: 'the page envelope',
    },
    {
      id: 'error-not-found',
      label: 'A miss, the GraphQL way',
      kind: 'error',
      query: 'query { post(id: "p-999") { id } }',
      note: 'null plus an errors entry',
    },
  ],
  runs: [
    {
      id: 'list-first-page',
      label: 'List the first page (paginated)',
      query: 'query { posts(page: 1, limit: 2) { items { id } } }',
      data: { posts: { items: [{ id: 'p-1' }], totalItems: 6 } },
      errors: [],
      notes: ['deterministic seed'],
    },
    {
      id: 'error-not-found',
      label: 'A miss, the GraphQL way',
      query: 'query { post(id: "p-999") { id } }',
      data: { post: null },
      errors: [{ message: 'Post "p-999" not found.', extensions: { code: 'not_found' } }],
      notes: [],
    },
  ],
  loader: {
    postCount: 6,
    loadsIssued: 12,
    duplicateLoads: 6,
    uniqueIds: 6,
    naivePortCalls: 7,
    naiveBreakdown: { list: 1, getById: 6, create: 0, replace: 0, delete: 0 },
    loaderPortCalls: 1,
    loaderBreakdown: { list: 1, getById: 0, create: 0, replace: 0, delete: 0 },
    allResolved: true,
    verdict: 'one batched call instead of seven',
  },
};

function stubDemo(options: {
  runResult?: { data: Record<string, unknown> | null; errors: { message: string; extensions?: Record<string, unknown> }[] };
}): void {
  container.initialize(createAppConfig());
  container.register(API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE, {
    execute: () => Promise.resolve({ ok: true, value: payload }),
  } as unknown as GetGraphQLPlaygroundDemoUseCase);
  container.register(API_GRAPHQL_DEMO_QUERY_USE_CASE, {
    execute: (query: string) => {
      ranQueries.push(query);
      return Promise.resolve({
        ok: true,
        value: options.runResult ?? { data: { posts: { totalItems: 6 } }, errors: [] },
      });
    },
  } as unknown as ExecuteGraphQLDemoQueryUseCase);
}

const ranQueries: string[] = [];

afterEach(() => {
  vi.clearAllMocks();
});

describe('GraphQLPlaygroundDemo', () => {
  it('renders the endpoint card, schema signatures, and the loader verdict', async () => {
    stubDemo({});

    render(<GraphQLPlaygroundDemo />);

    await waitFor(() => expect(screen.getByTestId('gqlp-endpoint')).toBeInTheDocument());

    const endpoint = screen.getByTestId('gqlp-endpoint');
    expect(endpoint).toHaveTextContent('POST /api/graphql');
    expect(endpoint).toHaveTextContent('"query"');

    expect(screen.getByTestId('gqlp-schema')).toHaveTextContent(
      'query posts(page: Int, limit: Int, tag: String, sort: String): PostPage!',
    );

    const loader = screen.getByTestId('gqlp-loader');
    expect(loader).toHaveTextContent('7');
    expect(loader).toHaveTextContent('one batched call instead of seven');
  });

  it('selects a sample, shows the reference run, and RUNs the exact document live', async () => {
    stubDemo({ runResult: { data: { post: null }, errors: [{ message: 'Post "p-999" not found.', extensions: { code: 'not_found' } }] } });

    render(<GraphQLPlaygroundDemo />);

    await waitFor(() => expect(screen.getByTestId('gqlp-samples')).toBeInTheDocument());

    // The reference run for the default selection is visible.
    expect(screen.getByTestId('gqlp-console')).toHaveTextContent('totalItems');

    fireEvent.click(screen.getByTestId('gqlp-sample-error-not-found'));
    expect(screen.getByTestId('gqlp-console')).toHaveTextContent('post(id: "p-999")');

    fireEvent.click(screen.getByTestId('gqlp-run-button'));

    await waitFor(() => expect(screen.getByTestId('gqlp-run-result')).toBeInTheDocument());
    expect(ranQueries).toContain('query { post(id: "p-999") { id } }');

    const live = screen.getByTestId('gqlp-run-result');
    expect(live).toHaveTextContent('Post "p-999" not found.');
    // The live answer's error list (scoped — the reference run has one too).
    expect(within(live).getByTestId('gqlp-errors')).toHaveTextContent('not_found');
  });
});
