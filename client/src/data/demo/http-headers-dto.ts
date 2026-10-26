import type {
  EchoedHeader,
  HeaderRole,
  HeaderScope,
  HttpHeadersDemoData,
} from '../../domain/demo/http-headers';
import {
  isRecord,
  optionalString,
  requiredArray,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * HttpHeadersDTO — the wire/transport shape of the headers echo payload.
 * Mirrors the domain model 1:1 (the interface is the seam where wire and
 * domain may diverge); the `fromJson` builds ONLY the listed fields
 * (whitelist) and throws on any shape violation, which parseDto upgrades
 * to an InvalidResponseException.
 *
 * `value` is deliberately `requiredText` (present-but-empty passes): a
 * header sent with no value (`curl -H 'X-Empty;'`) echoes as `""` — empty
 * is DATA there, not absence. `note` is optionalString because the server
 * OMITS the wire key when a header has no teaching note (optionals
 * omitted, never null). `scope` and `role` validate against the server's
 * closed vocabularies so a drifting classifier fails here first.
 */

export type HttpHeadersDemoDto = HttpHeadersDemoData;

const HEADER_SCOPES: readonly string[] = ['end-to-end', 'hop-by-hop'];

const HEADER_ROLES: readonly string[] = [
  'negotiation',
  'cors',
  'security',
  'identity',
  'extension',
  'transport',
  'other',
];

/** Deserialize + validate the GET /api/demo/http/headers payload. */
export function httpHeadersDtoFromJson(body: unknown): HttpHeadersDemoDto {
  if (!isRecord(body)) throw new Error('http-headers payload must be an object');
  return {
    method: requiredString(body, 'method'),
    path: requiredString(body, 'path'),
    query: optionalString(body, 'query'),
    proto: requiredString(body, 'proto'),
    requestID: requiredString(body, 'requestID'),
    contentType: optionalString(body, 'contentType'),
    headers: requiredArray(body, 'headers').map(echoedHeaderFromJson),
    hopByHop: requiredArray(body, 'hopByHop').map((name) => {
      if (typeof name !== 'string' || name === '') {
        throw new Error('expected a non-empty hop-by-hop header name');
      }
      return name;
    }),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toHttpHeadersDemo(dto: HttpHeadersDemoDto): HttpHeadersDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function echoedHeaderFromJson(body: unknown): EchoedHeader {
  if (!isRecord(body)) throw new Error('echoed header must be an object');
  const scope = requiredString(body, 'scope');
  const role = requiredString(body, 'role');
  if (!HEADER_SCOPES.includes(scope)) {
    throw new Error(`unknown header scope "${scope}"`);
  }
  if (!HEADER_ROLES.includes(role)) {
    throw new Error(`unknown header role "${role}"`);
  }
  return {
    name: requiredString(body, 'name'),
    value: requiredText(body, 'value'),
    scope: scope as HeaderScope,
    role: role as HeaderRole,
    note: optionalString(body, 'note'),
  };
}
