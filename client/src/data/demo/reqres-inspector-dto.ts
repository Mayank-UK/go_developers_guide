import type {
  ReqresDecodeExhibit,
  ReqresDecodeOutcome,
  ReqresDisciplineFacts,
  ReqresInspectorDemoData,
  ReqresInspectorRequest,
  ReqresLiveBody,
  ReqresLiveOutcome,
} from '../../domain/demo/reqres-inspector';
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
 * ReqresInspectorDTO — the wire/transport shape of the inspector
 * payload. Mirrors the domain model 1:1 (the interface is the seam where
 * wire and domain may diverge); the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException.
 *
 * `decoded` and `body` are deliberately `requiredText` (present-but-empty
 * passes): a truncated body decodes NOTHING — `decoded: ""` is the fact,
 * not absence. Optional wire keys (`contentType`, `error`, `field`, …)
 * are OMITTED by the server when absent — they must land as undefined,
 * never null. `outcome` validates against the server's closed
 * vocabularies so a drifting builder fails here first.
 */

export type ReqresInspectorDemoDto = ReqresInspectorDemoData;

const DECODE_OUTCOMES: readonly string[] = ['accepted', 'rejected'];
const LIVE_OUTCOMES: readonly string[] = ['accepted', 'rejected', 'none'];

/** Deserialize + validate the GET /api/demo/reqres/inspector payload. */
export function reqresInspectorDtoFromJson(body: unknown): ReqresInspectorDemoDto {
  if (!isRecord(body)) throw new Error('reqres-inspector payload must be an object');
  return {
    request: requestFromJson(body, 'request'),
    discipline: disciplineFromJson(body, 'discipline'),
    exhibits: requiredArray(body, 'exhibits').map(exhibitFromJson),
    liveBody: liveBodyFromJson(body, 'liveBody'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toReqresInspectorDemo(dto: ReqresInspectorDemoDto): ReqresInspectorDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function requestFromJson(body: Record<string, unknown>, key: string): ReqresInspectorRequest {
  const request = requiredRecord(body, key);
  return {
    method: requiredString(request, 'method'),
    path: requiredString(request, 'path'),
    contentType: optionalString(request, 'contentType'),
    bodyBytes: requiredNumber(request, 'bodyBytes'),
    bodyLimit: requiredNumber(request, 'bodyLimit'),
    bodyLimitExceeded: optionalBoolean(request, 'bodyLimitExceeded'),
  };
}

function disciplineFromJson(body: Record<string, unknown>, key: string): ReqresDisciplineFacts {
  const discipline = requiredRecord(body, key);
  return {
    decoder: requiredString(discipline, 'decoder'),
    unknownFields: requiredString(discipline, 'unknownFields'),
    trailing: requiredString(discipline, 'trailing'),
    firstError: requiredString(discipline, 'firstError'),
  };
}

function exhibitFromJson(body: unknown): ReqresDecodeExhibit {
  if (!isRecord(body)) throw new Error('decode exhibit must be an object');
  const outcome = requiredString(body, 'outcome');
  if (!DECODE_OUTCOMES.includes(outcome)) {
    throw new Error(`unknown decode outcome "${outcome}"`);
  }
  return {
    label: requiredString(body, 'label'),
    body: requiredText(body, 'body'),
    outcome: outcome as ReqresDecodeOutcome,
    error: optionalString(body, 'error'),
    errorType: optionalString(body, 'errorType'),
    field: optionalString(body, 'field'),
    unexpectedEOF: optionalBoolean(body, 'unexpectedEOF'),
    decoded: requiredText(body, 'decoded'),
  };
}

function liveBodyFromJson(body: Record<string, unknown>, key: string): ReqresLiveBody {
  const live = requiredRecord(body, key);
  const outcome = requiredString(live, 'outcome');
  if (!LIVE_OUTCOMES.includes(outcome)) {
    throw new Error(`unknown live-body outcome "${outcome}"`);
  }
  return {
    sent: requiredBoolean(live, 'sent'),
    body: optionalString(live, 'body'),
    outcome: outcome as ReqresLiveOutcome,
    error: optionalString(live, 'error'),
    errorType: optionalString(live, 'errorType'),
    field: optionalString(live, 'field'),
    unexpectedEOF: optionalBoolean(live, 'unexpectedEOF'),
    decoded: optionalString(live, 'decoded'),
  };
}

/**
 * optionalBoolean — the boolean twin of optionalString: absent means
 * undefined, present must be a boolean (never a truthy 1 or "true").
 */
function optionalBoolean(
  source: Record<string, unknown>,
  key: string,
): boolean | undefined {
  const value = source[key];
  if (value === undefined) return undefined;
  if (typeof value !== 'boolean') throw new Error(`expected boolean at "${key}"`);
  return value;
}
