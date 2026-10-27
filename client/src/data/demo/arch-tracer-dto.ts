import type {
  ArchLayer,
  ArchTraceHop,
  ArchTracePanel,
  ArchTracerDemoData,
  ArchViolationPanel,
} from '../../domain/demo/arch-tracer';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ArchTracerDTO — the wire/transport shape of the tracer payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException. The layer discriminates
 * against the server's closed vocabulary
 * (middleware|presentation|domain|data) and the dataMode against
 * (fake|live) — every duration, status, and count stays
 * server-derived, never pinned to literals here.
 */

export type ArchTracerDemoDto = ArchTracerDemoData;

const ARCH_LAYERS = new Set(['middleware', 'presentation', 'domain', 'data']);
const ARCH_DATA_MODES = new Set(['fake', 'live']);

function hopFromJson(value: unknown): ArchTraceHop {
  if (!isRecord(value)) throw new Error('arch trace hop must be an object');
  const layer = requiredString(value, 'layer');
  if (!ARCH_LAYERS.has(layer)) {
    throw new Error(`unknown arch trace layer "${layer}" (expected middleware|presentation|domain|data)`);
  }
  return {
    order: requiredNumber(value, 'order'),
    layer: layer as ArchLayer,
    file: requiredString(value, 'file'),
    symbol: requiredString(value, 'symbol'),
    note: requiredString(value, 'note'),
    durationMs: requiredNumber(value, 'durationMs'),
  };
}

function traceFromJson(body: Record<string, unknown>): ArchTracePanel {
  const trace = requiredRecord(body, 'trace');
  const dataMode = requiredString(trace, 'dataMode');
  if (!ARCH_DATA_MODES.has(dataMode)) {
    throw new Error(`unknown arch trace dataMode "${dataMode}" (expected fake|live)`);
  }
  return {
    request: requiredString(trace, 'request'),
    dataMode: dataMode as ArchTracePanel['dataMode'],
    leaf: requiredString(trace, 'leaf'),
    flipNote: requiredString(trace, 'flipNote'),
    hops: requiredArray(trace, 'hops').map(hopFromJson),
    status: requiredNumber(trace, 'status'),
    items: requiredNumber(trace, 'items'),
    elapsedMs: requiredNumber(trace, 'elapsedMs'),
    requestId: requiredString(trace, 'requestId'),
    note: requiredString(trace, 'note'),
  };
}

function violationFromJson(body: Record<string, unknown>): ArchViolationPanel {
  const violation = requiredRecord(body, 'violation');
  return {
    label: requiredString(violation, 'label'),
    snippet: requiredString(violation, 'snippet'),
    why: requiredString(violation, 'why'),
    caughtBy: requiredString(violation, 'caughtBy'),
    honest: requiredString(violation, 'honest'),
  };
}

/** Deserialize + validate the GET /api/demo/arch/tracer payload. */
export function archTracerDtoFromJson(body: unknown): ArchTracerDemoDto {
  if (!isRecord(body)) throw new Error('arch tracer payload must be an object');
  return {
    trace: traceFromJson(body),
    violation: violationFromJson(body),
    // Present-but-empty is real data: reasoning is server prose.
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toArchTracerDemo(dto: ArchTracerDemoDto): ArchTracerDemoData {
  return dto;
}
