import type {
  ErrorsKindRow,
  ErrorsProbePanel,
  ErrorsScenarioPanel,
  ErrorsExplorerDemoData,
  ErrorsTrigger,
  ErrorsWalkRow,
} from '../../domain/demo/errors-explorer';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
} from '../content/validate';

/**
 * ErrorsExplorerDTO — the wire/transport shape of the explorer payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException. Trigger modes validate
 * against the server's closed vocabulary (live|recipe) and the walk's
 * injection mode against the scenario enum (none|error|latency) — every
 * status, message, and note is server-derived, validated as non-empty
 * strings, never pinned to literals.
 */

export type ErrorsExplorerDemoDto = ErrorsExplorerDemoData;

const TRIGGER_MODES = new Set(['live', 'recipe']);
const INJECTION_MODES = new Set(['none', 'error', 'latency']);

function optionalString(source: Record<string, unknown>, key: string): string | undefined {
  if (source[key] === undefined) return undefined;
  const value = source[key];
  if (typeof value !== 'string' || value === '') {
    throw new Error(`expected a non-empty string at "${key}"`);
  }
  return value;
}

function triggerFromJson(value: unknown): ErrorsTrigger {
  if (!isRecord(value)) throw new Error('errors trigger must be an object');
  const mode = requiredString(value, 'mode');
  if (!TRIGGER_MODES.has(mode)) {
    throw new Error(`unknown errors trigger mode "${mode}" (expected live|recipe)`);
  }
  return {
    mode: mode as ErrorsTrigger['mode'],
    method: requiredString(value, 'method'),
    path: requiredString(value, 'path'),
    body: optionalString(value, 'body'),
    prep: optionalString(value, 'prep'),
    note: requiredString(value, 'note'),
  };
}

function kindFromJson(value: unknown): ErrorsKindRow {
  if (!isRecord(value)) throw new Error('errors kind row must be an object');
  return {
    kind: requiredString(value, 'kind'),
    httpStatus: requiredNumber(value, 'httpStatus'),
    defaultMessage: requiredString(value, 'defaultMessage'),
    raisedBy: requiredString(value, 'raisedBy'),
    loggedAs: requiredString(value, 'loggedAs'),
    trigger: triggerFromJson(value.trigger),
  };
}

function scenarioFromJson(body: Record<string, unknown>): ErrorsScenarioPanel {
  const scenario = requiredRecord(body, 'scenario');
  return {
    getEndpoint: requiredString(scenario, 'getEndpoint'),
    patchEndpoint: requiredString(scenario, 'patchEndpoint'),
    modes: requiredArray(scenario, 'modes').map((value) => {
      if (typeof value !== 'string' || value === '') {
        throw new Error('expected a non-empty string at "scenario.modes"');
      }
      return value;
    }),
    note: requiredString(scenario, 'note'),
  };
}

function probeFromJson(body: Record<string, unknown>): ErrorsProbePanel {
  const probe = requiredRecord(body, 'probe');
  return {
    probeEndpoint: requiredString(probe, 'probeEndpoint'),
    latencyMillis: requiredNumber(probe, 'latencyMillis'),
    healthyNote: requiredString(probe, 'healthyNote'),
    injectedNote: requiredString(probe, 'injectedNote'),
    latencyNote: requiredString(probe, 'latencyNote'),
  };
}

function walkFromJson(body: Record<string, unknown>): ErrorsWalkRow {
  const walk = requiredRecord(body, 'walk');
  const injectFailure = requiredString(walk, 'injectFailure');
  if (!INJECTION_MODES.has(injectFailure)) {
    throw new Error(
      `unknown errors walk injection "${injectFailure}" (expected none|error|latency)`,
    );
  }
  return {
    injectFailure,
    status: requiredNumber(walk, 'status'),
    elapsedMillis: requiredNumber(walk, 'elapsedMillis'),
    requestID: requiredString(walk, 'requestID'),
    body: requiredString(walk, 'body'),
    note: requiredString(walk, 'note'),
  };
}

/** Deserialize + validate the GET /api/demo/errors/explorer payload. */
export function errorsExplorerDtoFromJson(body: unknown): ErrorsExplorerDemoDto {
  if (!isRecord(body)) throw new Error('errors explorer payload must be an object');
  return {
    kinds: requiredArray(body, 'kinds').map(kindFromJson),
    envelopeNote: requiredString(body, 'envelopeNote'),
    scenario: scenarioFromJson(body),
    probe: probeFromJson(body),
    walk: walkFromJson(body),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toErrorsExplorerDemo(dto: ErrorsExplorerDemoDto): ErrorsExplorerDemoData {
  return dto;
}
