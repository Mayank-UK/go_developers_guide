import type {
  ChainLayer,
  MiddlewarePipelineDemoData,
  PanicRecoveryBeat,
  RequestIDBeat,
  TimeoutBeat,
  TraceEvent,
  TracePhase,
} from '../../domain/demo/middleware-pipeline';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * MiddlewarePipelineDTO — the wire/transport shape of the middleware
 * pipeline payload. Mirrors the domain model 1:1 (the interface is the
 * seam where wire and domain may diverge); the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException.
 *
 * `requestIDSeen` is deliberately `requiredText` (present-but-empty
 * passes): the trace's first beat fires BEFORE the request-id layer has
 * assigned anything, so `""` is data — the beat's whole teaching point —
 * not absence. `phase` validates against the server's closed vocabulary
 * so a drifting builder fails here first.
 */

export type MiddlewarePipelineDemoDto = MiddlewarePipelineDemoData;

const TRACE_PHASES: readonly string[] = ['before', 'handler', 'after'];

/** Deserialize + validate the GET /api/demo/middleware/pipeline payload. */
export function middlewarePipelineDtoFromJson(body: unknown): MiddlewarePipelineDemoDto {
  if (!isRecord(body)) throw new Error('middleware-pipeline payload must be an object');
  return {
    chain: requiredArray(body, 'chain').map(chainLayerFromJson),
    trace: requiredArray(body, 'trace').map(traceEventFromJson),
    requestSideOrder: nameListFromJson(body, 'requestSideOrder'),
    responseSideOrder: nameListFromJson(body, 'responseSideOrder'),
    requestID: requestIDBeatFromJson(requiredRecord(body, 'requestID')),
    panicRecovery: panicRecoveryBeatFromJson(requiredRecord(body, 'panicRecovery')),
    timeoutBeat: timeoutBeatFromJson(requiredRecord(body, 'timeoutBeat')),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toMiddlewarePipelineDemo(dto: MiddlewarePipelineDemoDto): MiddlewarePipelineDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function chainLayerFromJson(body: unknown): ChainLayer {
  if (!isRecord(body)) throw new Error('chain layer must be an object');
  return {
    name: requiredString(body, 'name'),
    position: requiredNumber(body, 'position'),
    role: requiredString(body, 'role'),
    mirrors: requiredString(body, 'mirrors'),
  };
}

function traceEventFromJson(body: unknown): TraceEvent {
  if (!isRecord(body)) throw new Error('trace event must be an object');
  const phase = requiredString(body, 'phase');
  if (!TRACE_PHASES.includes(phase)) {
    throw new Error(`unknown trace phase "${phase}"`);
  }
  return {
    layer: requiredString(body, 'layer'),
    phase: phase as TracePhase,
    sequence: requiredNumber(body, 'sequence'),
    requestIDSeen: requiredText(body, 'requestIDSeen'),
    note: requiredString(body, 'note'),
  };
}

function nameListFromJson(body: Record<string, unknown>, key: string): string[] {
  return requiredArray(body, key).map((name) => {
    if (typeof name !== 'string' || name === '') {
      throw new Error(`expected a non-empty layer name at "${key}"`);
    }
    return name;
  });
}

function requestIDBeatFromJson(body: Record<string, unknown>): RequestIDBeat {
  return {
    headerName: requiredString(body, 'headerName'),
    inbound: requiredString(body, 'inbound'),
    seenAtLayers: nameListFromJson(body, 'seenAtLayers'),
    responseHeader: requiredString(body, 'responseHeader'),
    sameIDEverywhere: requiredBoolean(body, 'sameIDEverywhere'),
  };
}

function panicRecoveryBeatFromJson(body: Record<string, unknown>): PanicRecoveryBeat {
  return {
    path: requiredString(body, 'path'),
    cause: requiredString(body, 'cause'),
    status: requiredNumber(body, 'status'),
    body: requiredText(body, 'body'),
    loggedLine: requiredText(body, 'loggedLine'),
    requestIDKept: requiredBoolean(body, 'requestIDKept'),
    trace: requiredArray(body, 'trace').map(traceEventFromJson),
  };
}

function timeoutBeatFromJson(body: Record<string, unknown>): TimeoutBeat {
  return {
    wrapper: requiredString(body, 'wrapper'),
    budgetMs: requiredNumber(body, 'budgetMs'),
    handlerSleepMs: requiredNumber(body, 'handlerSleepMs'),
    observedStatus: requiredNumber(body, 'observedStatus'),
    body: requiredText(body, 'body'),
    slowHandlerFinished: requiredBoolean(body, 'slowHandlerFinished'),
    shape: requiredString(body, 'shape'),
  };
}
