import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type {
  GraphQLPlaygroundRepository,
  GraphQLQueryRepository,
} from '../../domain/demo/graphql-playground-repository';
import type {
  GraphQLExecutionResult,
  GraphQLPlaygroundDemoData,
} from '../../domain/demo/graphql-playground';
import { toGraphQLPlaygroundDemo } from './graphql-playground-dto';
import type { GraphQLPlaygroundApi } from './graphql-playground-api';

/**
 * GraphQLPlaygroundRepositoryImpl — adapts the demo API to BOTH domain
 * ports (the payload port and the RUN port share one transport because
 * they share one server). Every thrown transport error (network, non-2xx,
 * malformed payload) is guarded by safeRepositoryCall into a Failed
 * Result, so the demo component never needs try/catch and no rejection
 * goes unhandled.
 */
export class GraphQLPlaygroundRepositoryImpl
  implements GraphQLPlaygroundRepository, GraphQLQueryRepository
{
  private readonly api: GraphQLPlaygroundApi;

  constructor(api: GraphQLPlaygroundApi) {
    this.api = api;
  }

  async get(): Promise<Result<GraphQLPlaygroundDemoData>> {
    return safeRepositoryCall(async () => toGraphQLPlaygroundDemo(await this.api.get()));
  }

  async run(query: string): Promise<Result<GraphQLExecutionResult>> {
    return safeRepositoryCall(async () => this.api.run(query));
  }
}
