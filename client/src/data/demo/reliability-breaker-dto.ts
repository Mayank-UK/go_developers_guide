import type {
  ReliabilityAttemptView,
  ReliabilityBreakerDemoData,
  ReliabilityBreakerPanel,
  ReliabilityConfigPanel,
  ReliabilityCounters,
  ReliabilityRetryPanel,
  ReliabilityRetryRowView,
  ReliabilityScenarioPanel,
  ReliabilityTransitionView,
  ReliabilityWalkPanel,
} from '../../domain/demo/reliability-breaker';
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
 * ReliabilityBreakerDTO — the wire/transport shape of the breaker
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY
 * the listed fields (whitelist) and throws on any shape violation,
 * which parseDto upgrades to an InvalidResponseException.
 */

export type ReliabilityBreakerDemoDto = ReliabilityBreakerDemoData;

function scenarioFromJson(value: unknown): ReliabilityScenarioPanel {
  if (!isRecord(value)) throw new Error('scenario panel must be an object');
  return {
    mode: requiredString(value, 'mode'),
    getEndpoint: requiredText(value, 'getEndpoint'),
    patchEndpoint: requiredText(value, 'patchEndpoint'),
    modes: requiredArray(value, 'modes').map((mode) => {
      if (typeof mode !== 'string' || mode === '') {
        throw new Error('scenario modes must be non-empty strings');
      }
      return mode;
    }),
    note: requiredText(value, 'note'),
  };
}

function configFromJson(value: unknown): ReliabilityConfigPanel {
  if (!isRecord(value)) throw new Error('breaker config must be an object');
  return {
    failureThreshold: requiredNumber(value, 'failureThreshold'),
    cooldownMs: requiredNumber(value, 'cooldownMs'),
    callTimeoutMs: requiredNumber(value, 'callTimeoutMs'),
    walkAttempts: requiredNumber(value, 'walkAttempts'),
    halfOpenProbes: requiredNumber(value, 'halfOpenProbes'),
    retryAttempts: requiredNumber(value, 'retryAttempts'),
    retryBaseDelayMs: requiredNumber(value, 'retryBaseDelayMs'),
    retryJitterMs: requiredNumber(value, 'retryJitterMs'),
  };
}

function countersFromJson(value: unknown): ReliabilityCounters {
  if (!isRecord(value)) throw new Error('breaker counters must be an object');
  return {
    allowed: requiredNumber(value, 'allowed'),
    fastFailed: requiredNumber(value, 'fastFailed'),
    succeeded: requiredNumber(value, 'succeeded'),
    failed: requiredNumber(value, 'failed'),
  };
}

function transitionFromJson(value: unknown): ReliabilityTransitionView {
  if (!isRecord(value)) throw new Error('breaker transition must be an object');
  return {
    at: requiredString(value, 'at'),
    from: requiredString(value, 'from'),
    to: requiredString(value, 'to'),
    cause: requiredText(value, 'cause'),
  };
}

function breakerFromJson(value: unknown): ReliabilityBreakerPanel {
  if (!isRecord(value)) throw new Error('breaker panel must be an object');
  return {
    state: requiredString(value, 'state'),
    stateAgeMs: requiredNumber(value, 'stateAgeMs'),
    config: configFromJson(requiredRecord(value, 'config')),
    counters: countersFromJson(requiredRecord(value, 'counters')),
    probeInFlight: requiredBoolean(value, 'probeInFlight'),
    transitions: requiredArray(value, 'transitions').map(transitionFromJson),
  };
}

function attemptFromJson(value: unknown): ReliabilityAttemptView {
  if (!isRecord(value)) throw new Error('walk attempt must be an object');
  return {
    index: requiredNumber(value, 'index'),
    stateBefore: requiredString(value, 'stateBefore'),
    action: requiredString(value, 'action'),
    outcome: requiredString(value, 'outcome'),
    detail: optionalString(value, 'detail'),
    elapsedMs: requiredNumber(value, 'elapsedMs'),
    stateAfter: requiredString(value, 'stateAfter'),
  };
}

function walkFromJson(value: unknown): ReliabilityWalkPanel {
  if (!isRecord(value)) throw new Error('walk panel must be an object');
  return {
    mode: requiredString(value, 'mode'),
    rows: requiredArray(value, 'rows').map(attemptFromJson),
    note: requiredText(value, 'note'),
  };
}

function retryRowFromJson(value: unknown): ReliabilityRetryRowView {
  if (!isRecord(value)) throw new Error('retry row must be an object');
  return {
    attempt: requiredNumber(value, 'attempt'),
    outcome: requiredString(value, 'outcome'),
    detail: optionalString(value, 'detail'),
    elapsedMs: requiredNumber(value, 'elapsedMs'),
    backoffMs: requiredNumber(value, 'backoffMs'),
  };
}

function retryFromJson(value: unknown): ReliabilityRetryPanel {
  if (!isRecord(value)) throw new Error('retry panel must be an object');
  return {
    mode: requiredString(value, 'mode'),
    succeeded: requiredBoolean(value, 'succeeded'),
    rows: requiredArray(value, 'rows').map(retryRowFromJson),
    totalElapsedMs: requiredNumber(value, 'totalElapsedMs'),
    note: requiredText(value, 'note'),
  };
}

/** Deserialize + validate the GET /api/demo/reliability/breaker payload. */
export function reliabilityBreakerDtoFromJson(body: unknown): ReliabilityBreakerDemoDto {
  if (!isRecord(body)) throw new Error('reliability breaker payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    scenario: scenarioFromJson(requiredRecord(body, 'scenario')),
    breaker: breakerFromJson(requiredRecord(body, 'breaker')),
    walk: walkFromJson(requiredRecord(body, 'walk')),
    retry: retryFromJson(requiredRecord(body, 'retry')),
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toReliabilityBreakerDemo(dto: ReliabilityBreakerDemoDto): ReliabilityBreakerDemoData {
  return dto;
}
