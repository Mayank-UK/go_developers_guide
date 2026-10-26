import type {
  ReqresMethodsDemoData,
  ReqresMethodRow,
  ReqresObservedHeader,
  ReqresSSEBeat,
  ReqresSSEEvent,
} from '../../domain/demo/reqres-methods';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ReqresMethodsDTO — the wire/transport shape of the methods report.
 * Mirrors the domain model 1:1 (the interface is the seam where wire and
 * domain may diverge); the `fromJson` builds ONLY the listed fields
 * (whitelist) and throws on any shape violation, which parseDto upgrades
 * to an InvalidResponseException.
 *
 * `body` is deliberately `requiredText` (present-but-empty passes): the
 * 204/HEAD/OPTIONS rows carry no body by protocol rule — empty is the
 * FACT there, not absence. The SSE event `data` is the payload verbatim
 * (never re-parsed here — the server reports what the recorder saw).
 */

export type ReqresMethodsDemoDto = ReqresMethodsDemoData;

/** Deserialize + validate the GET /api/demo/reqres/methods payload. */
export function reqresMethodsDtoFromJson(body: unknown): ReqresMethodsDemoDto {
  if (!isRecord(body)) throw new Error('reqres-methods payload must be an object');
  return {
    resource: requiredString(body, 'resource'),
    allow: requiredString(body, 'allow'),
    methods: requiredArray(body, 'methods').map(methodRowFromJson),
    sse: sseFromJson(body, 'sse'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toReqresMethodsDemo(dto: ReqresMethodsDemoDto): ReqresMethodsDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function methodRowFromJson(body: unknown): ReqresMethodRow {
  if (!isRecord(body)) throw new Error('method row must be an object');
  const status = requiredNumber(body, 'status');
  if (!Number.isInteger(status) || status < 100 || status > 599) {
    throw new Error(`method row status ${status} is not an HTTP status`);
  }
  return {
    method: requiredString(body, 'method'),
    status,
    statusText: requiredString(body, 'statusText'),
    headers: requiredArray(body, 'headers').map(observedHeaderFromJson),
    bodyShape: requiredString(body, 'bodyShape'),
    body: requiredText(body, 'body'),
  };
}

function observedHeaderFromJson(body: unknown): ReqresObservedHeader {
  if (!isRecord(body)) throw new Error('observed header must be an object');
  return {
    name: requiredString(body, 'name'),
    value: requiredText(body, 'value'),
  };
}

function sseFromJson(body: Record<string, unknown>, key: string): ReqresSSEBeat {
  const sse = requiredRecord(body, key);
  const events = requiredArray(sse, 'events').map(sseEventFromJson);
  const flushes = requiredNumber(sse, 'flushes');
  if (!Number.isInteger(flushes) || flushes < 0) {
    throw new Error(`SSE flush count ${flushes} is not a count`);
  }
  return {
    contentType: requiredString(sse, 'contentType'),
    events,
    flushes,
    framing: requiredString(sse, 'framing'),
  };
}

function sseEventFromJson(body: unknown): ReqresSSEEvent {
  if (!isRecord(body)) throw new Error('SSE event must be an object');
  return {
    name: requiredString(body, 'name'),
    data: requiredString(body, 'data'),
  };
}
