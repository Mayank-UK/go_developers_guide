import type {
  ObsMalformedRow,
  ObsSpanRow,
  ObsTraceAccepted,
  ObsTraceDemoData,
} from '../../domain/demo/obs-trace';
import {
  isRecord,
  optionalString,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ObsTraceDTO — the wire/transport shape of the trace round-trip
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY
 * the listed fields (whitelist) and throws on any shape violation,
 * which parseDto upgrades to an InvalidResponseException. Ids stay
 * server-derived strings (never re-validated as hex here — the server
 * already did that work, and the client re-deriving it would be a
 * second source of truth).
 */

export type ObsTraceDemoDto = ObsTraceDemoData;

function acceptedFromJson(body: Record<string, unknown>): ObsTraceAccepted {
  const accepted = requiredRecord(body, 'accepted');
  return {
    // Present-but-empty is real data: inbound is "" when the request
    // carried no traceparent (the origin note says it was minted).
    inbound: requiredText(accepted, 'inbound'),
    inboundOrigin: requiredString(accepted, 'inboundOrigin'),
    version: requiredString(accepted, 'version'),
    traceId: requiredString(accepted, 'traceId'),
    parentId: requiredString(accepted, 'parentId'),
    flags: requiredString(accepted, 'flags'),
    sampled: requiredBoolean(accepted, 'sampled'),
    tracestate: optionalString(accepted, 'tracestate'),
    responseEcho: requiredString(accepted, 'responseEcho'),
    note: requiredString(accepted, 'note'),
  };
}

function spanRowFromJson(value: unknown): ObsSpanRow {
  if (!isRecord(value)) throw new Error('span row must be an object');
  return {
    name: requiredString(value, 'name'),
    layer: requiredString(value, 'layer'),
    spanId: requiredString(value, 'spanId'),
    parentId: requiredString(value, 'parentId'),
    traceId: requiredString(value, 'traceId'),
    depth: requiredNumber(value, 'depth'),
    startOffsetMs: requiredNumber(value, 'startOffsetMs'),
    durationMs: requiredNumber(value, 'durationMs'),
    note: requiredString(value, 'note'),
  };
}

function malformedRowFromJson(value: unknown): ObsMalformedRow {
  if (!isRecord(value)) throw new Error('malformed row must be an object');
  return {
    header: requiredString(value, 'header'),
    problem: requiredString(value, 'problem'),
    handledAs: requiredString(value, 'handledAs'),
  };
}

/** Deserialize + validate the GET /api/demo/obs/trace payload. */
export function obsTraceDtoFromJson(body: unknown): ObsTraceDemoDto {
  if (!isRecord(body)) throw new Error('obs trace payload must be an object');
  return {
    accepted: acceptedFromJson(body),
    tree: requiredArray(body, 'tree').map(spanRowFromJson),
    malformed: requiredArray(body, 'malformed').map(malformedRowFromJson),
    // Present-but-empty is real data: the SDK comparison is server prose.
    sdk: requiredText(body, 'sdk'),
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toObsTraceDemo(dto: ObsTraceDemoDto): ObsTraceDemoData {
  return dto;
}
