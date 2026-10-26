import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { GraphQLExecutionResult } from './graphql-playground';
import type { GraphQLQueryRepository } from './graphql-playground-repository';
import { ExecuteGraphQLDemoQueryUseCase } from './execute-graphql-demo-query-usecase';

/**
 * ExecuteGraphQLDemoQueryUseCase spec — the RUN button's seam. Pin the
 * two failure layers the way the demo renders them: GraphQL errors ride a
 * SUCCESS (the operation executed; the graph answered), while a broken
 * transport arrives as a Failed Result, and neither ever throws.
 */

function fakeRepository(result: Result<GraphQLExecutionResult>): GraphQLQueryRepository {
  return {
    run: () => Promise.resolve(result),
  };
}

describe('ExecuteGraphQLDemoQueryUseCase', () => {
  it('passes the query text to the repository and returns its result', async () => {
    const answer: GraphQLExecutionResult = {
      data: { posts: { totalItems: 6 } },
      errors: [],
    };
    const seen: string[] = [];
    const repository: GraphQLQueryRepository = {
      run: (query) => {
        seen.push(query);
        return Promise.resolve(success(answer));
      },
    };
    const useCase = new ExecuteGraphQLDemoQueryUseCase(repository);

    const result = await useCase.execute('query { posts { totalItems } }');

    expect(seen).toEqual(['query { posts { totalItems } }']);
    expect(result).toEqual(success(answer));
  });

  it('returns GraphQL errors inside a SUCCESS — the operation executed', async () => {
    const miss: GraphQLExecutionResult = {
      data: { post: null },
      errors: [{ message: 'Post "p-999" not found.', extensions: { code: 'not_found' } }],
    };
    const useCase = new ExecuteGraphQLDemoQueryUseCase(fakeRepository(success(miss)));

    const result = await useCase.execute('query { post(id: "p-999") { id } }');

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.errors[0]?.extensions?.['code']).toBe('not_found');
      expect(result.value.data).toEqual({ post: null });
    }
  });

  it('returns a Failed Result when the transport breaks (never throws)', async () => {
    const useCase = new ExecuteGraphQLDemoQueryUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute('query { posts { totalItems } }');

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
    }
  });
});
