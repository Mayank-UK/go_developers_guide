import { describe, expect, it } from 'vitest';
import { jobsConsoleDtoFromJson, toJobsConsoleDemo } from './jobs-console-dto';
import type { JobsConsoleDemoData } from '../../domain/demo/jobs-console';

/**
 * JobsConsoleDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream
 * as an invalid_response Failure, never as a guessed default). The
 * namespace, task ids, and proof sentences are live randomness per run
 * — asserted by shape only, never pinned to literals.
 */

const wellFormed: JobsConsoleDemoData = {
  namespace: 'gdg:demo:jobs:0123456789ab',
  queues: [
    { name: 'demos', weight: 6 },
    { name: 'low', weight: 1 },
  ],
  commands: ['ENQUEUE', 'RETRY', 'ARCHIVE', 'RUN', 'DELETE', 'REGISTER', 'STATS'],
  tasks: [
    {
      id: 'gdg:demo:jobs:0123456789ab:flaky',
      type: 'demo:flaky',
      queue: 'demos',
      maxRetry: 2,
      attempts: [
        { attempt: 1, outcome: 'failed' },
        { attempt: 2, outcome: 'failed' },
        { attempt: 3, outcome: 'succeeded' },
      ],
      verdict: 'succeeded after 3 attempt(s) — the retry ladder worked as designed',
    },
    {
      id: 'gdg:demo:jobs:0123456789ab:poison',
      type: 'demo:poison',
      queue: 'low',
      maxRetry: 1,
      attempts: [
        { attempt: 1, outcome: 'failed' },
        { attempt: 2, outcome: 'failed' },
      ],
      verdict: 'failed every attempt — archived, replayed once, re-archived, then deleted',
    },
  ],
  beats: [
    {
      beat: 'enqueue',
      commands: ['ENQUEUE demo:flaky … Queue(demos) MaxRetry(2)'],
      outcome: 'two tasks accepted by the broker.',
    },
    {
      beat: 'retry',
      commands: ['RETRY gdg:demo:jobs:0123456789ab:flaky ← exponential backoff per failure'],
      outcome: 'the flaky task\u2019s attempt ladder: 1:failed → 2:failed → 3:succeeded.',
    },
  ],
  cron: { spec: '@every 1s', ticks: 2 },
  observability: { peakDepth: 3, peakOldestAgeMs: 640, finalDepth: 0, finalOldestAgeMs: 0 },
  proofs: {
    retry: 'the flaky task failed 2 attempt(s) under exponential backoff and succeeded on attempt 3.',
    archive: 'the poison task failed all 2 of its budgeted attempts and the archive now holds it.',
    replay: 'RunTask moved the archived task back to pending; the ledger grew by 1 attempt.',
    cron: 'the scheduler registered "@every 1s" and the ledger observed 2 tick task(s).',
    observability: 'the queues\u2019 combined depth peaked at 3 with the oldest pending task aged 640 ms.',
  },
  reasoning: 'failure is a first-class state — the task comes back with a growing delay.',
};

describe('jobsConsoleDtoFromJson', () => {
  it('parses the exact wire shape', () => {
    expect(jobsConsoleDtoFromJson(structuredClone(wellFormed))).toEqual(wellFormed);
  });

  it('rejects non-objects', () => {
    expect(() => jobsConsoleDtoFromJson(null)).toThrow();
    expect(() => jobsConsoleDtoFromJson('jobs')).toThrow();
    expect(() => jobsConsoleDtoFromJson([])).toThrow();
  });

  it('rejects missing or malformed tasks', () => {
    const missingTasks: unknown = { ...wellFormed, tasks: undefined };
    expect(() => jobsConsoleDtoFromJson(missingTasks)).toThrow();

    const emptyId: unknown = {
      ...wellFormed,
      tasks: [{ ...wellFormed.tasks[0]!, id: '' }],
    };
    expect(() => jobsConsoleDtoFromJson(emptyId)).toThrow();

    const emptyAttempts: unknown = {
      ...wellFormed,
      tasks: [{ ...wellFormed.tasks[0]!, attempts: undefined }],
    };
    expect(() => jobsConsoleDtoFromJson(emptyAttempts)).toThrow();
  });

  it('rejects an attempt outcome outside the closed vocabulary', () => {
    const bogusOutcome: unknown = {
      ...wellFormed,
      tasks: [
        { ...wellFormed.tasks[0]!, attempts: [{ attempt: 1, outcome: 'pending' }] },
      ],
    };
    expect(() => jobsConsoleDtoFromJson(bogusOutcome)).toThrow(/failed\|succeeded/);
  });

  it('rejects missing or malformed observability and cron', () => {
    const noObservability: unknown = { ...wellFormed, observability: undefined };
    expect(() => jobsConsoleDtoFromJson(noObservability)).toThrow();

    const stringDepth: unknown = {
      ...wellFormed,
      observability: { ...wellFormed.observability, peakDepth: '3' },
    };
    expect(() => jobsConsoleDtoFromJson(stringDepth)).toThrow();

    const noCron: unknown = { ...wellFormed, cron: undefined };
    expect(() => jobsConsoleDtoFromJson(noCron)).toThrow();
  });

  it('rejects malformed beats and proofs', () => {
    const emptyCommand: unknown = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0]!, commands: [''] }],
    };
    expect(() => jobsConsoleDtoFromJson(emptyCommand)).toThrow();

    const noProofs: unknown = { ...wellFormed, proofs: undefined };
    expect(() => jobsConsoleDtoFromJson(noProofs)).toThrow();

    const missingProofField: unknown = {
      ...wellFormed,
      proofs: { ...wellFormed.proofs, replay: undefined },
    };
    expect(() => jobsConsoleDtoFromJson(missingProofField)).toThrow();
  });

  it('converts to the domain model identically while 1:1', () => {
    const dto = jobsConsoleDtoFromJson(structuredClone(wellFormed));
    expect(toJobsConsoleDemo(dto)).toEqual(wellFormed);
  });
});
