import type {
  GRPCCallReport,
  GRPCConsoleDemoData,
  GRPCMethodKind,
  GRPCResultRow,
  GRPCStatusName,
  GRPCTransportFacts,
} from '../../domain/demo/grpc-console';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * GRPCConsoleDTO — the wire/transport shape of the grpc console payload.
 * Mirrors the domain model 1:1 (the interface is the seam where wire and
 * domain may diverge); the `fromJson` builds ONLY the listed fields
 * (whitelist) and throws on any shape violation, which parseDto upgrades
 * to an InvalidResponseException.
 *
 * `detail` is deliberately `requiredText` (present-but-empty passes): the
 * server's rows carry observed values and an empty detail would be data
 * about the RPC, not absence. `kind` and `statusCode` validate against
 * the server's closed vocabularies so a drifting exhibit fails here
 * first, and `ok: false` marks every rejection row (a miss, an expired
 * deadline, a rejected ack) — the console's whole point is that error
 * CODES are data.
 */

export type GRPCConsoleDemoDto = GRPCConsoleDemoData;

const METHOD_KINDS: readonly string[] = ['unary', 'server-stream', 'client-stream', 'bidi'];

const STATUS_NAMES: readonly string[] = [
  'OK',
  'NotFound',
  'InvalidArgument',
  'DeadlineExceeded',
  'Unavailable',
];

/** Deserialize + validate the GET /api/demo/api/grpc-console payload. */
export function grpcConsoleDtoFromJson(body: unknown): GRPCConsoleDemoDto {
  if (!isRecord(body)) throw new Error('grpc-console payload must be an object');
  const calls = requiredArray(body, 'calls').map(callFromJson);
  if (calls.length === 0) {
    throw new Error('the console always carries exhibit calls — an empty list is a broken payload');
  }
  return {
    transport: transportFromJson(body.transport),
    calls,
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toGRPCConsoleDemo(dto: GRPCConsoleDemoDto): GRPCConsoleDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function transportFromJson(body: unknown): GRPCTransportFacts {
  if (!isRecord(body)) throw new Error('transport facts must be an object');
  return {
    mode: requiredText(body, 'mode'),
    dialTarget: requiredText(body, 'dialTarget'),
    security: requiredText(body, 'security'),
    protoFile: requiredText(body, 'protoFile'),
    service: requiredText(body, 'service'),
    generatedGo: requiredText(body, 'generatedGo'),
    codegenCmd: requiredText(body, 'codegenCmd'),
  };
}

function callFromJson(body: unknown): GRPCCallReport {
  if (!isRecord(body)) throw new Error('call report must be an object');
  const kind = requiredString(body, 'kind');
  const statusCode = requiredString(body, 'statusCode');
  if (!METHOD_KINDS.includes(kind)) {
    throw new Error(`unknown method kind "${kind}"`);
  }
  if (!STATUS_NAMES.includes(statusCode)) {
    throw new Error(`unknown status code "${statusCode}"`);
  }
  return {
    kind: kind as GRPCMethodKind,
    method: requiredText(body, 'method'),
    pattern: requiredText(body, 'pattern'),
    summary: requiredText(body, 'summary'),
    statusCode: statusCode as GRPCStatusName,
    statusNumber: requiredNumber(body, 'statusNumber'),
    roundTrips: requiredNumber(body, 'roundTrips'),
    results: requiredArray(body, 'results').map(resultRowFromJson),
  };
}

function resultRowFromJson(body: unknown): GRPCResultRow {
  if (!isRecord(body)) throw new Error('result row must be an object');
  return {
    label: requiredText(body, 'label'),
    detail: requiredText(body, 'detail'),
    ok: requiredBoolean(body, 'ok'),
  };
}
