import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { GraphQLPlaygroundDemoData } from './graphql-playground';
import type { GraphQLPlaygroundRepository } from './graphql-playground-repository';
import { GetGraphQLPlaygroundDemoUseCase } from './get-graphql-playground-demo-usecase';

/**
 * GetGraphQLPlaygroundDemoUseCase spec — the use case is a thin
 * pass-through seam, so the contract to pin is exactly that: whatever the
 * repository resolves (Success or Failed) arrives unchanged, and nothing
 * throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<GraphQLPlaygroundDemoData>): GraphQLPlaygroundRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: GraphQLPlaygroundDemoData = {
  endpoint: {
    path: '/api/graphql',
    method: 'POST',
    requestBody: '{"query":"{ posts(limit: 2) { items { id title } } }"}',
    liveNote: 'runs execute the same generated executor in-process',
  },
  schema: {
    operations: ['query posts(page: Int, limit: Int, tag: String, sort: String): PostPage!'],
    types: [{ name: 'Post', kind: 'object', fields: [{ name: 'id', type: 'ID!' }] }],
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
    verdict: 'one batched call',
  },
};

describe('GetGraphQLPlaygroundDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetGraphQLPlaygroundDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetGraphQLPlaygroundDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
