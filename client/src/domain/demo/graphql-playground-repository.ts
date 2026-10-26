import type { Result } from '../../core/result';
import type {
  GraphQLExecutionResult,
  GraphQLPlaygroundDemoData,
} from './graphql-playground';

/**
 * GraphQLPlaygroundRepository — the domain-facing port for the playground
 * demo payload. Demo components depend on this interface only; the HTTP
 * adapter lives in data/demo and is wired at the composition root.
 */
export interface GraphQLPlaygroundRepository {
  /** The executed runs, schema card, samples, and loader experiment. */
  get(): Promise<Result<GraphQLPlaygroundDemoData>>;
}

/**
 * GraphQLQueryRepository — the port for the demo's RUN button: one
 * operation text, executed against the LIVE POST /api/graphql endpoint.
 * Separate from the payload port because it is a different data source
 * (the real graph, not the deterministic companion payload).
 */
export interface GraphQLQueryRepository {
  /** Execute one GraphQL document; GraphQL errors ride the 200's body. */
  run(query: string): Promise<Result<GraphQLExecutionResult>>;
}
