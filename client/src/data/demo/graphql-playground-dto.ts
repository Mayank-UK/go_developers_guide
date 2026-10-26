import type {
  GraphQLErrorView,
  GraphQLExecutionResult,
  GraphQLFieldCard,
  GraphQLLoaderExperiment,
  GraphQLPlaygroundDemoData,
  GraphQLPortBreakdown,
  GraphQLRunResult,
  GraphQLSample,
  GraphQLSampleKind,
  GraphQLSchemaCard,
  GraphQLTypeCard,
} from '../../domain/demo/graphql-playground';
import {
  isRecord,
  optionalRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * GraphQLPlaygroundDTO — the wire/transport shape of the playground
 * payload. Mirrors the domain model 1:1 (the interface is the seam where
 * wire and domain may diverge); `fromJson` builds ONLY the listed fields
 * (whitelist) and throws on any shape violation, which parseDto upgrades
 * to an InvalidResponseException.
 *
 * Nullability notes, straight from the server's builder:
 * - `message` on an error view is `requiredText` (present-but-empty
 *   passes) — an empty message would be data, not absence.
 * - A run's (or a live result's) `data` is an object OR the JSON null —
 *   legitimate GraphQL: execution can fail before data assembles. Null
 *   here is a VALUE the demo renders, so it flows through as null.
 * - `errors` is always an array (never null); `extensions` is omitted
 *   when absent, never null.
 */

export type GraphQLPlaygroundDemoDto = GraphQLPlaygroundDemoData;

const SAMPLE_KINDS: readonly string[] = ['query', 'mutation', 'introspection', 'error'];

/** Deserialize + validate the GET /api/demo/api/graphql-playground payload. */
export function graphqlPlaygroundDtoFromJson(body: unknown): GraphQLPlaygroundDemoDto {
  if (!isRecord(body)) throw new Error('graphql-playground payload must be an object');
  return {
    endpoint: endpointFrom(body),
    schema: schemaFrom(body),
    samples: samplesFrom(body),
    runs: runsFrom(body),
    loader: loaderFrom(body),
  };
}

/** Deserialize + validate one live execution result (POST /api/graphql body). */
export function graphqlExecutionFromJson(body: unknown): GraphQLExecutionResult {
  if (!isRecord(body)) throw new Error('graphql response must be an object');
  return {
    data: dataDocument(body),
    errors: errorViews(body),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toGraphQLPlaygroundDemo(dto: GraphQLPlaygroundDemoDto): GraphQLPlaygroundDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function endpointFrom(body: Record<string, unknown>) {
  const endpoint = requiredRecord(body, 'endpoint');
  return {
    path: requiredString(endpoint, 'path'),
    method: requiredString(endpoint, 'method'),
    requestBody: requiredString(endpoint, 'requestBody'),
    liveNote: requiredString(endpoint, 'liveNote'),
  };
}

function schemaFrom(body: Record<string, unknown>): GraphQLSchemaCard {
  const schema = requiredRecord(body, 'schema');
  return {
    operations: requiredArray(schema, 'operations').map(nonEmptyString('operations entry')),
    types: requiredArray(schema, 'types').map(typeCardFromJson),
  };
}

function typeCardFromJson(body: unknown): GraphQLTypeCard {
  if (!isRecord(body)) throw new Error('schema type card must be an object');
  return {
    name: requiredString(body, 'name'),
    kind: requiredString(body, 'kind'),
    fields: requiredArray(body, 'fields').map((field) => {
      if (!isRecord(field)) throw new Error('field card must be an object');
      const card: GraphQLFieldCard = {
        name: requiredString(field, 'name'),
        type: requiredString(field, 'type'),
      };
      return card;
    }),
  };
}

function samplesFrom(body: Record<string, unknown>): GraphQLSample[] {
  return requiredArray(body, 'samples').map((sample) => {
    if (!isRecord(sample)) throw new Error('sample must be an object');
    const kind = requiredString(sample, 'kind');
    if (!SAMPLE_KINDS.includes(kind)) {
      throw new Error(`unknown sample kind "${kind}"`);
    }
    return {
      id: requiredString(sample, 'id'),
      label: requiredString(sample, 'label'),
      kind: kind as GraphQLSampleKind,
      query: requiredString(sample, 'query'),
      note: requiredString(sample, 'note'),
    };
  });
}

function runsFrom(body: Record<string, unknown>): GraphQLRunResult[] {
  return requiredArray(body, 'runs').map((run) => {
    if (!isRecord(run)) throw new Error('run must be an object');
    return {
      id: requiredString(run, 'id'),
      label: requiredString(run, 'label'),
      query: requiredString(run, 'query'),
      data: runDataFromJson(run['data']),
      errors: errorViews(run),
      notes: requiredArray(run, 'notes').map(nonEmptyString('note')),
    };
  });
}

/** A run's data document: an object, or the JSON null (a VALUE, not absence). */
function runDataFromJson(value: unknown): Record<string, unknown> | null {
  if (value === null) return null;
  if (!isRecord(value)) throw new Error('run data must be an object or null');
  return value;
}

function dataDocument(body: Record<string, unknown>): Record<string, unknown> | null {
  if (!('data' in body)) throw new Error('graphql response needs a data member');
  return runDataFromJson(body['data']);
}

function errorViews(body: Record<string, unknown>): GraphQLErrorView[] {
  const errors = requiredArray(body, 'errors');
  return errors.map((entry) => {
    if (!isRecord(entry)) throw new Error('error view must be an object');
    return {
      message: requiredText(entry, 'message'),
      extensions: optionalRecord(entry, 'extensions'),
    };
  });
}

function loaderFrom(body: Record<string, unknown>): GraphQLLoaderExperiment {
  const loader = requiredRecord(body, 'loader');
  return {
    postCount: requiredNumber(loader, 'postCount'),
    loadsIssued: requiredNumber(loader, 'loadsIssued'),
    duplicateLoads: requiredNumber(loader, 'duplicateLoads'),
    uniqueIds: requiredNumber(loader, 'uniqueIds'),
    naivePortCalls: requiredNumber(loader, 'naivePortCalls'),
    naiveBreakdown: breakdownFromJson(loader, 'naiveBreakdown'),
    loaderPortCalls: requiredNumber(loader, 'loaderPortCalls'),
    loaderBreakdown: breakdownFromJson(loader, 'loaderBreakdown'),
    allResolved: requiredBoolean(loader, 'allResolved'),
    verdict: requiredText(loader, 'verdict'),
  };
}

function breakdownFromJson(body: Record<string, unknown>, key: string): GraphQLPortBreakdown {
  const breakdown = requiredRecord(body, key);
  return {
    list: requiredNumber(breakdown, 'list'),
    getById: requiredNumber(breakdown, 'getById'),
    create: requiredNumber(breakdown, 'create'),
    replace: requiredNumber(breakdown, 'replace'),
    delete: requiredNumber(breakdown, 'delete'),
  };
}

function nonEmptyString(what: string): (value: unknown) => string {
  return (value) => {
    if (typeof value !== 'string' || value === '') {
      throw new Error(`expected a non-empty ${what}`);
    }
    return value;
  };
}
