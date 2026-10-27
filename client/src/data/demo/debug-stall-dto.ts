import type {
  DebugStallCapture,
  DebugStallDemoData,
  DebugStallEndpoint,
  DebugStallFrame,
  DebugStallStack,
  DebugStallWorkload,
} from '../../domain/demo/debug-stall';
import {
  isRecord,
  optionalString,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * DebugStallDTO — the wire/transport shape of the stall payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException. Optional wire fields (a
 * goroutine's wait, createdBy, createdAt) map to omitted-when-absent —
 * never null. The "(not found)" empty state arrives as a plain state
 * string — the DTO does not invent that marker, the server does.
 */

export type DebugStallDemoDto = DebugStallDemoData;

function frameFromJson(value: unknown): DebugStallFrame {
  if (!isRecord(value)) throw new Error('frame must be an object');
  return {
    function: requiredString(value, 'function'),
    location: requiredString(value, 'location'),
  };
}

function stackFromJson(value: unknown): DebugStallStack {
  if (!isRecord(value)) throw new Error('stack row must be an object');
  return {
    id: requiredNumber(value, 'id'),
    state: requiredString(value, 'state'),
    wait: optionalString(value, 'wait'),
    frames: requiredArray(value, 'frames').map(frameFromJson),
    createdBy: optionalString(value, 'createdBy'),
    createdAt: optionalString(value, 'createdAt'),
  };
}

function workloadFromJson(body: Record<string, unknown>): DebugStallWorkload {
  const workload = requiredRecord(body, 'workload');
  return {
    workers: requiredNumber(workload, 'workers'),
    healthyDone: requiredNumber(workload, 'healthyDone'),
    wedge: requiredString(workload, 'wedge'),
  };
}

function captureFromJson(body: Record<string, unknown>): DebugStallCapture {
  const capture = requiredRecord(body, 'capture');
  return {
    profile: requiredString(capture, 'profile'),
    goroutinesSeen: requiredNumber(capture, 'goroutinesSeen'),
    captureMs: requiredNumber(capture, 'captureMs'),
    captureTries: requiredNumber(capture, 'captureTries'),
    curl: requiredString(capture, 'curl'),
  };
}

function endpointFromJson(value: unknown): DebugStallEndpoint {
  if (!isRecord(value)) throw new Error('endpoint must be an object');
  return {
    path: requiredString(value, 'path'),
    profile: requiredString(value, 'profile'),
    note: requiredString(value, 'note'),
  };
}

/** Deserialize + validate the GET /api/demo/debug/stall payload. */
export function debugStallDtoFromJson(body: unknown): DebugStallDemoDto {
  if (!isRecord(body)) throw new Error('debug stall payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    workload: workloadFromJson(body),
    capture: captureFromJson(body),
    stalled: stackFromJson(requiredRecord(body, 'stalled')),
    contrast: stackFromJson(requiredRecord(body, 'contrast')),
    endpoints: requiredArray(body, 'endpoints').map(endpointFromJson),
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toDebugStallDemo(dto: DebugStallDemoDto): DebugStallDemoData {
  return dto;
}
