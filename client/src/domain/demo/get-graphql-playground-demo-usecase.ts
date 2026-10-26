import type { Result } from '../../core/result';
import type { GraphQLPlaygroundRepository } from './graphql-playground-repository';
import type { GraphQLPlaygroundDemoData } from './graphql-playground';

/**
 * API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE — the container token this demo's
 * use case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol.
 */
export const API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE = Symbol('GetGraphQLPlaygroundDemoUseCase');

/**
 * GetGraphQLPlaygroundDemoUseCase — loads the graphql topic's playground
 * payload (the in-process executed runs, the schema card, the samples,
 * and the loader experiment). Thin orchestration seam over the repository
 * port (react-guide use-case style); parameterless because the payload
 * is the server's deterministic description of its own surface.
 */
export class GetGraphQLPlaygroundDemoUseCase {
  private readonly repository: GraphQLPlaygroundRepository;

  constructor(repository: GraphQLPlaygroundRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<GraphQLPlaygroundDemoData>> {
    return this.repository.get();
  }
}
