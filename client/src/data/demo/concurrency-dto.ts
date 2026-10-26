import type {
  CancelBeat,
  ConcurrencyDemoData,
  JobRow,
  PoolSetup,
  RuntimeFacts,
  TimeoutBeat,
  WorkerRow,
} from '../../domain/demo/concurrency';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ConcurrencyDemoDTO — the wire/transport shape of the concurrency demo
 * payload. Mirrors the domain model 1:1 (the interface is the seam where
 * wire and domain may diverge); the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException.
 *
 * One field is deliberately `requiredText` (present-but-empty passes)
 * rather than `requiredString`: a timeout beat's `received` is `""` when
 * the timer wins — empty is DATA there, not absence. Every other string
 * in this payload is provably non-empty on the wire (zero-padded job ids,
 * worker ids, outcomes), so they stay `requiredString`.
 */

export type ConcurrencyDemoDto = ConcurrencyDemoData;

/** Deserialize + validate the GET /api/demo/runtime/concurrency payload. */
export function concurrencyDtoFromJson(body: unknown): ConcurrencyDemoDto {
  if (!isRecord(body)) throw new Error('concurrency payload must be an object');
  return {
    setup: setupFromJson(body.setup),
    fanOut: requiredArray(body, 'fanOut').map(jobRowFromJson),
    workerCounts: requiredArray(body, 'workerCounts').map(workerRowFromJson),
    cancellation: cancelBeatFromJson(body.cancellation),
    timeouts: requiredArray(body, 'timeouts').map(timeoutBeatFromJson),
    runtime: runtimeFactsFromJson(body.runtime),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toConcurrencyDemo(dto: ConcurrencyDemoDto): ConcurrencyDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function setupFromJson(body: unknown): PoolSetup {
  if (!isRecord(body)) throw new Error('pool setup must be an object');
  return {
    jobs: requiredNumber(body, 'jobs'),
    workers: requiredNumber(body, 'workers'),
    cancelAfter: requiredNumber(body, 'cancelAfter'),
    strategy: requiredString(body, 'strategy'),
  };
}

function jobRowFromJson(body: unknown): JobRow {
  if (!isRecord(body)) throw new Error('job row must be an object');
  return {
    jobId: requiredString(body, 'jobId'),
    payload: requiredString(body, 'payload'),
    output: requiredString(body, 'output'),
  };
}

function workerRowFromJson(body: unknown): WorkerRow {
  if (!isRecord(body)) throw new Error('worker row must be an object');
  return {
    workerId: requiredString(body, 'workerId'),
    jobsDone: requiredNumber(body, 'jobsDone'),
  };
}

function cancelBeatFromJson(body: unknown): CancelBeat {
  if (!isRecord(body)) throw new Error('cancel beat must be an object');
  return {
    jobsSubmitted: requiredNumber(body, 'jobsSubmitted'),
    resultsCollected: requiredNumber(body, 'resultsCollected'),
    jobsAbandoned: requiredNumber(body, 'jobsAbandoned'),
    workersStopped: requiredNumber(body, 'workersStopped'),
    outcome: requiredString(body, 'outcome'),
  };
}

function timeoutBeatFromJson(body: unknown): TimeoutBeat {
  if (!isRecord(body)) throw new Error('timeout beat must be an object');
  return {
    scenario: requiredString(body, 'scenario'),
    producerDelayMs: requiredNumber(body, 'producerDelayMs'),
    timeoutMs: requiredNumber(body, 'timeoutMs'),
    winner: requiredString(body, 'winner'),
    received: requiredText(body, 'received'),
  };
}

function runtimeFactsFromJson(body: unknown): RuntimeFacts {
  if (!isRecord(body)) throw new Error('runtime facts must be an object');
  return {
    gomaxprocs: requiredNumber(body, 'gomaxprocs'),
    goroutinesBefore: requiredNumber(body, 'goroutinesBefore'),
    goroutinesDuring: requiredNumber(body, 'goroutinesDuring'),
    goroutinesAfter: requiredNumber(body, 'goroutinesAfter'),
  };
}
