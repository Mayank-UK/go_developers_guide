/**
 * Domain models for the graphql topic's playground demo — the client-side
 * shape of `GET /api/demo/api/graphql-playground`. Field names match the
 * server's camelCase wire contract 1:1 (the DTO is the seam where wire
 * and domain may diverge later).
 */

/** A sample operation's kind — the server's closed vocabulary. */
export type GraphQLSampleKind = 'query' | 'mutation' | 'introspection' | 'error';

/** The transport card: where the real endpoint lives, one request body. */
export interface GraphQLEndpointMeta {
  path: string;
  method: string;
  requestBody: string;
  liveNote: string;
}

/** One field of a schema type, rendered as the schema language writes it. */
export interface GraphQLFieldCard {
  name: string;
  type: string;
}

/** One named type from the real parsed schema (object or input object). */
export interface GraphQLTypeCard {
  name: string;
  kind: string;
  fields: GraphQLFieldCard[];
}

/** The schema summary: root operation signatures + the domain types. */
export interface GraphQLSchemaCard {
  operations: string[];
  types: GraphQLTypeCard[];
}

/** One selectable playground operation, query text included. */
export interface GraphQLSample {
  id: string;
  label: string;
  kind: GraphQLSampleKind;
  query: string;
  note: string;
}

/**
 * One `errors[]` entry the surface can answer: a human message plus the
 * taxonomy-carrying extensions (`code`, and for validation failures the
 * `details` issue list). Absent when the entry carries none.
 */
export interface GraphQLErrorView {
  message: string;
  extensions?: Record<string, unknown>;
}

/**
 * One sample operation EXECUTED in-process by the server: the data
 * document verbatim (an object, or null when execution failed before
 * data assembled — legitimate GraphQL, not absence), the errors list
 * (never null), and the determinism notes.
 */
export interface GraphQLRunResult {
  id: string;
  label: string;
  query: string;
  data: Record<string, unknown> | null;
  errors: GraphQLErrorView[];
  notes: string[];
}

/** The instrumented port's reading — one counter per port method. */
export interface GraphQLPortBreakdown {
  list: number;
  getById: number;
  create: number;
  replace: number;
  delete: number;
}

/** The N+1-versus-dataloader experiment, as numbers the demo renders. */
export interface GraphQLLoaderExperiment {
  postCount: number;
  loadsIssued: number;
  duplicateLoads: number;
  uniqueIds: number;
  naivePortCalls: number;
  naiveBreakdown: GraphQLPortBreakdown;
  loaderPortCalls: number;
  loaderBreakdown: GraphQLPortBreakdown;
  allResolved: boolean;
  verdict: string;
}

/** The whole playground payload. */
export interface GraphQLPlaygroundDemoData {
  endpoint: GraphQLEndpointMeta;
  schema: GraphQLSchemaCard;
  samples: GraphQLSample[];
  runs: GraphQLRunResult[];
  loader: GraphQLLoaderExperiment;
}

/**
 * The response document of one operation run against the LIVE endpoint —
 * GraphQL's single response shape: data present but possibly null, the
 * errors list possibly empty. A transport failure never looks like this;
 * it arrives as a typed Failure inside the Result instead.
 */
export interface GraphQLExecutionResult {
  data: Record<string, unknown> | null;
  errors: GraphQLErrorView[];
}
