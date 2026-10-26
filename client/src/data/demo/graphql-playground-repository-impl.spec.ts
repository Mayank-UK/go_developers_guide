import { afterEach, describe, expect, it, vi } from 'vitest';
import { NetworkFailure } from '../../core/error-handling/failures';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { GraphQLPlaygroundApi } from './graphql-playground-api';
import { GraphQLPlaygroundRepositoryImpl } from './graphql-playground-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * GraphQLPlaygroundRepositoryImpl spec — fixture-driven (happy + failure),
 * covering BOTH ports: the payload GET and the live RUN POST. GraphQL
 * errors riding a 200 body are a SUCCESS (they are data); transport
 * failures resolve as Failed Results; nothing ever throws.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  endpoint: {
    path: '/api/graphql',
    method: 'POST',
    requestBody: '{"query":"{ posts(limit: 2) { items { id title } } }"}',
    liveNote: 'runs execute the same generated executor in-process',
  },
  schema: {
    operations: ['query posts(page: Int, limit: Int, tag: String, sort: String): PostPage!'],
    types: [
      { name: 'Post', kind: 'object', fields: [{ name: 'id', type: 'ID!' }] },
      { name: 'PostInput', kind: 'input', fields: [{ name: 'title', type: 'String!' }] },
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
  ],
  runs: [
    {
      id: 'list-first-page',
      label: 'List the first page (paginated)',
      query: 'query { posts(page: 1, limit: 2) { items { id } } }',
      data: { posts: { items: [{ id: 'p-1' }], totalItems: 6 } },
      errors: [],
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

function makeRepository(): GraphQLPlaygroundRepositoryImpl {
  return new GraphQLPlaygroundRepositoryImpl(new GraphQLPlaygroundApi(createAppConfig()));
}

describe('GraphQLPlaygroundRepositoryImpl', () => {
  it('returns a Success carrying the validated payload', async () => {
    stubApiFetch([{ url: '/api/demo/api/graphql-playground', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.endpoint.path).toBe('/api/graphql');
      expect(result.value.loader.naivePortCalls).toBe(7);
      expect(result.value.samples[0]?.query).toContain('posts(');
    }
  });

  it('returns a Success for a live run whose errors ride the 200 body', async () => {
    stubApiFetch([
      {
        url: '/api/graphql',
        method: 'POST',
        body: {
          data: { post: null },
          errors: [{ message: 'Post "p-999" not found.', extensions: { code: 'not_found' } }],
        },
      },
    ]);

    const result = await makeRepository().run('query { post(id: "p-999") { id } }');

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.data).toEqual({ post: null });
      expect(result.value.errors[0]?.extensions?.['code']).toBe('not_found');
    }
  });

  it('resolves a network failure as a Failed Result (never throws)', async () => {
    stubNetworkFailure();

    const payload = await makeRepository().get();
    expect(payload.ok).toBe(false);

    const run = await makeRepository().run('query { posts { totalItems } }');
    expect(run.ok).toBe(false);
    if (!run.ok) {
      expect(run.failure).toBeInstanceOf(NetworkFailure);
    }
  });

  it('resolves a malformed payload as a Failed Result', async () => {
    stubApiFetch([{ url: '/api/demo/api/graphql-playground', body: { unexpected: true } }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
  });
});
