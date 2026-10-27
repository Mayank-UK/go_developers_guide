import type {
  JobsAttempt,
  JobsConsoleBeat,
  JobsConsoleDemoData,
  JobsConsoleProofs,
  JobsCron,
  JobsObservability,
  JobsQueue,
  JobsTask,
} from '../../domain/demo/jobs-console';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
} from '../content/validate';

/**
 * JobsConsoleDTO — the wire/transport shape of the console-story
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException. Attempt outcomes
 * validate against the server's closed vocabulary (failed/succeeded),
 * and every namespace, id, and proof sentence is server-minted per run
 * — validated as non-empty strings, never pinned to literals.
 */

export type JobsConsoleDemoDto = JobsConsoleDemoData;

const OUTCOMES = new Set(['failed', 'succeeded']);

function stringArray(source: Record<string, unknown>, key: string): string[] {
  return requiredArray(source, key).map((value) => {
    if (typeof value !== 'string' || value === '') {
      throw new Error(`expected a non-empty string at "${key}"`);
    }
    return value;
  });
}

function queueFromJson(value: unknown): JobsQueue {
  if (!isRecord(value)) throw new Error('jobs queue must be an object');
  return {
    name: requiredString(value, 'name'),
    weight: requiredNumber(value, 'weight'),
  };
}

function attemptFromJson(value: unknown): JobsAttempt {
  if (!isRecord(value)) throw new Error('jobs attempt must be an object');
  const outcome = requiredString(value, 'outcome');
  if (!OUTCOMES.has(outcome)) {
    throw new Error(`unknown jobs attempt outcome "${outcome}" (expected failed|succeeded)`);
  }
  return {
    attempt: requiredNumber(value, 'attempt'),
    outcome,
  };
}

function taskFromJson(value: unknown): JobsTask {
  if (!isRecord(value)) throw new Error('jobs task must be an object');
  return {
    id: requiredString(value, 'id'),
    type: requiredString(value, 'type'),
    queue: requiredString(value, 'queue'),
    maxRetry: requiredNumber(value, 'maxRetry'),
    attempts: requiredArray(value, 'attempts').map(attemptFromJson),
    verdict: requiredString(value, 'verdict'),
  };
}

function beatFromJson(value: unknown): JobsConsoleBeat {
  if (!isRecord(value)) throw new Error('jobs console beat must be an object');
  return {
    beat: requiredString(value, 'beat'),
    commands: stringArray(value, 'commands'),
    outcome: requiredString(value, 'outcome'),
  };
}

function cronFromJson(body: Record<string, unknown>): JobsCron {
  const cron = requiredRecord(body, 'cron');
  return {
    spec: requiredString(cron, 'spec'),
    ticks: requiredNumber(cron, 'ticks'),
  };
}

function observabilityFromJson(body: Record<string, unknown>): JobsObservability {
  const obs = requiredRecord(body, 'observability');
  return {
    peakDepth: requiredNumber(obs, 'peakDepth'),
    peakOldestAgeMs: requiredNumber(obs, 'peakOldestAgeMs'),
    finalDepth: requiredNumber(obs, 'finalDepth'),
    finalOldestAgeMs: requiredNumber(obs, 'finalOldestAgeMs'),
  };
}

function proofsFromJson(body: Record<string, unknown>): JobsConsoleProofs {
  const proofs = requiredRecord(body, 'proofs');
  return {
    retry: requiredString(proofs, 'retry'),
    archive: requiredString(proofs, 'archive'),
    replay: requiredString(proofs, 'replay'),
    cron: requiredString(proofs, 'cron'),
    observability: requiredString(proofs, 'observability'),
  };
}

/** Deserialize + validate the GET /api/demo/jobs/console payload. */
export function jobsConsoleDtoFromJson(body: unknown): JobsConsoleDemoDto {
  if (!isRecord(body)) throw new Error('jobs console payload must be an object');
  return {
    namespace: requiredString(body, 'namespace'),
    queues: requiredArray(body, 'queues').map(queueFromJson),
    commands: stringArray(body, 'commands'),
    tasks: requiredArray(body, 'tasks').map(taskFromJson),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    cron: cronFromJson(body),
    observability: observabilityFromJson(body),
    proofs: proofsFromJson(body),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toJobsConsoleDemo(dto: JobsConsoleDemoDto): JobsConsoleDemoData {
  return dto;
}
