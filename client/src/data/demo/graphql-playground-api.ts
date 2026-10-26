import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, guardedSendJson, parseDto } from '../../core/error-handling/guards';
import {
  graphqlExecutionFromJson,
  graphqlPlaygroundDtoFromJson,
  type GraphQLPlaygroundDemoDto,
} from './graphql-playground-dto';
import type { GraphQLExecutionResult } from '../../domain/demo/graphql-playground';

/**
 * GraphQLPlaygroundApi — the two transports the graphql topic's demo
 * drives, in the same style as the family's demo APIs:
 *
 * 1. `get` — GET /demo/api/graphql-playground, the deterministic
 *    companion payload (executed runs, schema card, loader experiment).
 * 2. `run` — POST /api/graphql, the REAL graph: one {query} document,
 *    answered with GraphQL's single response shape. The run always sends
 *    a well-formed document, so the endpoint's 400 tier (broken
 *    transport) is not expected here; execution errors ride the 200 body
 *    and are DATA, not transport failures.
 */

const DEMO_PATH = '/demo/api/graphql-playground';
/** Relative to the config's apiBaseUrl ('/api'): the graph's one endpoint. */
const GRAPHQL_PATH = '/graphql';

export class GraphQLPlaygroundApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/api/graphql-playground — the payload card. */
  async get(): Promise<GraphQLPlaygroundDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, graphqlPlaygroundDtoFromJson);
    });
  }

  /** POST {apiBaseUrl}/api/graphql — run one document against the live graph. */
  async run(query: string): Promise<GraphQLExecutionResult> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedSendJson(this.config, GRAPHQL_PATH, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ query }),
      });
      return parseDto(body, status, graphqlExecutionFromJson);
    });
  }
}
