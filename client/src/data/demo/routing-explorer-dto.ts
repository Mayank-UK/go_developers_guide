import type {
  ProbeOutcome,
  RouteKind,
  RoutingExplorerDemoData,
  RoutingExhibit,
  RoutingPattern,
  RoutingProbe,
  RoutingSemantics,
  RoutingValue,
  RoutingValueSource,
} from '../../domain/demo/routing-explorer';
import {
  isRecord,
  optionalArray,
  optionalString,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * RoutingExplorerDTO — the wire/transport shape of the routing explorer
 * payload. Mirrors the domain model 1:1 (the interface is the seam where
 * wire and domain may diverge); the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException.
 *
 * Closed vocabularies (`kind`, `outcome`, `source`) and the four truthful
 * statuses (200/405/404/307) validate against the Go builder's contract
 * so a drifting classifier fails here first. Optional wire keys
 * (`matchedPattern`, `pathValues`, `queryValues`, `allow`,
 * `redirectTarget`, and exhibit patterns' `handler`) are OMITTED by the
 * server when absent — they must land as undefined, never as null.
 * `reasoning` is deliberately `requiredText` (present-but-empty passes):
 * it is authored prose, and an empty sentence is a builder bug the spec
 * fixtures would surface, not absence of data.
 */

export type RoutingExplorerDemoDto = RoutingExplorerDemoData;

const ROUTE_KINDS: readonly string[] = ['exact', 'wildcard', 'subtree', 'trailing-wildcard'];

const PROBE_OUTCOMES: readonly string[] = ['matched', 'method_not_allowed', 'not_found', 'redirect'];

const VALUE_SOURCES: readonly string[] = ['path', 'query'];

const PROBE_STATUSES: readonly number[] = [200, 307, 404, 405];

/** Deserialize + validate the GET /api/demo/routing/explorer payload. */
export function routingExplorerDtoFromJson(body: unknown): RoutingExplorerDemoDto {
  if (!isRecord(body)) throw new Error('routing-explorer payload must be an object');
  return {
    routes: requiredArray(body, 'routes').map(routingPatternFromJson),
    probes: requiredArray(body, 'probes').map(routingProbeFromJson),
    exhibits: requiredArray(body, 'exhibits').map(routingExhibitFromJson),
    semantics: routingSemanticsFromJson(requiredRecordOrThrow(body, 'semantics')),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toRoutingExplorerDemo(dto: RoutingExplorerDemoDto): RoutingExplorerDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function routingPatternFromJson(body: unknown): RoutingPattern {
  if (!isRecord(body)) throw new Error('routing pattern must be an object');
  const kind = requiredString(body, 'kind');
  if (!ROUTE_KINDS.includes(kind)) {
    throw new Error(`unknown route kind "${kind}"`);
  }
  return {
    method: requiredString(body, 'method'),
    pattern: requiredString(body, 'pattern'),
    kind: kind as RouteKind,
    handler: optionalString(body, 'handler'),
  };
}

function routingValueFromJson(body: unknown): RoutingValue {
  if (!isRecord(body)) throw new Error('routing value must be an object');
  const source = requiredString(body, 'source');
  if (!VALUE_SOURCES.includes(source)) {
    throw new Error(`unknown routing value source "${source}"`);
  }
  const values = requiredArray(body, 'values').map((value) => {
    if (typeof value !== 'string') throw new Error('expected a string routing value');
    return value;
  });
  if (values.length === 0) {
    throw new Error('a routing value carries at least one occurrence');
  }
  return {
    name: requiredString(body, 'name'),
    values,
    source: source as RoutingValueSource,
  };
}

function routingProbeFromJson(body: unknown): RoutingProbe {
  if (!isRecord(body)) throw new Error('routing probe must be an object');
  const outcome = requiredString(body, 'outcome');
  if (!PROBE_OUTCOMES.includes(outcome)) {
    throw new Error(`unknown probe outcome "${outcome}"`);
  }
  const status = requiredNumber(body, 'status');
  if (!Number.isInteger(status) || !PROBE_STATUSES.includes(status)) {
    throw new Error(`probe status ${status} is not one of the truthful statuses`);
  }
  return {
    method: requiredString(body, 'method'),
    path: requiredString(body, 'path'),
    outcome: outcome as ProbeOutcome,
    status,
    matchedPattern: optionalString(body, 'matchedPattern'),
    pathValues: optionalArray(body, 'pathValues')?.map(routingValueFromJson),
    queryValues: optionalArray(body, 'queryValues')?.map(routingValueFromJson),
    allow: optionalArray(body, 'allow')?.map((method) => {
      if (typeof method !== 'string' || method === '') {
        throw new Error('expected a non-empty Allow entry');
      }
      return method;
    }),
    redirectTarget: optionalString(body, 'redirectTarget'),
    reasoning: requiredText(body, 'reasoning'),
  };
}

function routingExhibitFromJson(body: unknown): RoutingExhibit {
  if (!isRecord(body)) throw new Error('routing exhibit must be an object');
  return {
    id: requiredString(body, 'id'),
    title: requiredString(body, 'title'),
    note: requiredString(body, 'note'),
    patterns: requiredArray(body, 'patterns').map(routingPatternFromJson),
    probes: requiredArray(body, 'probes').map(routingProbeFromJson),
  };
}

function routingSemanticsFromJson(body: Record<string, unknown>): RoutingSemantics {
  const status = requiredNumber(body, 'subtreeRedirectStatus');
  if (!Number.isInteger(status) || status <= 0) {
    throw new Error('subtreeRedirectStatus must be a positive HTTP status');
  }
  return {
    getAlsoServesHead: requiredBoolean(body, 'getAlsoServesHead'),
    methodlessMatchesAll: requiredBoolean(body, 'methodlessMatchesAll'),
    subtreeRedirectStatus: status,
    reverseSlashRedirect: requiredBoolean(body, 'reverseSlashRedirect'),
    matchesEscapedPath: requiredBoolean(body, 'matchesEscapedPath'),
    pathValueIsUnescaped: requiredBoolean(body, 'pathValueIsUnescaped'),
  };
}

function requiredRecordOrThrow(source: Record<string, unknown>, key: string): Record<string, unknown> {
  const value = source[key];
  if (!isRecord(value)) throw new Error(`expected object at "${key}"`);
  return value;
}
