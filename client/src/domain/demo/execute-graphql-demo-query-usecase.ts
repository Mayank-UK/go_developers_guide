import type { Result } from '../../core/result';
import type { GraphQLQueryRepository } from './graphql-playground-repository';
import type { GraphQLExecutionResult } from './graphql-playground';

/**
 * API_GRAPHQL_DEMO_QUERY_USE_CASE — the container token the RUN button's
 * use case is registered under. Lives here for the same reason as the
 * payload token: the demo component resolves its seam from day one, and
 * the composition root registers the real pipeline under this symbol.
 */
export const API_GRAPHQL_DEMO_QUERY_USE_CASE = Symbol('ExecuteGraphQLDemoQueryUseCase');

/**
 * ExecuteGraphQLDemoQueryUseCase — the playground console's RUN button:
 * one GraphQL document, executed against the LIVE POST /api/graphql
 * endpoint. The result distinguishes two failure layers the way GraphQL
 * itself does: a Failed Result means the transport broke (network,
 * non-2xx); a Success carrying `errors` means the operation executed and
 * the graph answered with GraphQL errors — both render, honestly.
 */
export class ExecuteGraphQLDemoQueryUseCase {
  private readonly repository: GraphQLQueryRepository;

  constructor(repository: GraphQLQueryRepository) {
    this.repository = repository;
  }

  execute(query: string): Promise<Result<GraphQLExecutionResult>> {
    return this.repository.run(query);
  }
}
