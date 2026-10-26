import { describe, expect, it } from 'vitest';
import { concurrencyDtoFromJson, toConcurrencyDemo } from './concurrency-dto';
import type { ConcurrencyDemoData } from '../../domain/demo/concurrency';

/**
 * ConcurrencyDemoDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). The empty
 * `received` of a timer-won race — real data on the actual payload — must
 * survive validation, while a job id that vanished entirely must not.
 */

const wellFormed: ConcurrencyDemoData = {
  setup: { jobs: 8, workers: 3, cancelAfter: 5, strategy: 'worker pool: fan-out, fan-in, cancellation via select' },
  fanOut: [
    { jobId: 'job-01', payload: 'alpha', output: 'ALPHA#518' },
    { jobId: 'job-08', payload: 'hotel', output: 'HOTEL#531' },
  ],
  workerCounts: [
    { workerId: 'worker-1', jobsDone: 4 },
    { workerId: 'worker-2', jobsDone: 0 },
    { workerId: 'worker-3', jobsDone: 4 },
  ],
  cancellation: {
    jobsSubmitted: 8,
    resultsCollected: 5,
    jobsAbandoned: 3,
    workersStopped: 3,
    outcome: 'context cancelled after 5 of 8 results; 3 jobs abandoned, 3 workers returned',
  },
  timeouts: [
    { scenario: 'reply already waiting', producerDelayMs: 0, timeoutMs: 30, winner: 'reply', received: 'pong' },
    { scenario: 'reply arrives late', producerDelayMs: 200, timeoutMs: 30, winner: 'timer', received: '' },
  ],
  runtime: { gomaxprocs: 8, goroutinesBefore: 2, goroutinesDuring: 5, goroutinesAfter: 2 },
};

describe('concurrencyDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(concurrencyDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps the empty received of a timer-won race (empty is data)', () => {
    const parsed = concurrencyDtoFromJson(wellFormed);

    expect(parsed.timeouts[1]?.winner).toBe('timer');
    expect(parsed.timeouts[1]?.received).toBe('');
  });

  it('keeps a zero job tally (a worker may win no jobs this run)', () => {
    const parsed = concurrencyDtoFromJson(wellFormed);

    expect(parsed.workerCounts[1]?.jobsDone).toBe(0);
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing setup', { ...wellFormed, setup: undefined }],
    ['missing fanOut', { ...wellFormed, fanOut: undefined }],
    ['missing workerCounts', { ...wellFormed, workerCounts: undefined }],
    ['missing cancellation', { ...wellFormed, cancellation: undefined }],
    ['missing timeouts', { ...wellFormed, timeouts: undefined }],
    ['missing runtime', { ...wellFormed, runtime: undefined }],
    ['fanOut not an array', { ...wellFormed, fanOut: 'nope' }],
    ['a non-object job row', { ...wellFormed, fanOut: ['nope'] }],
    ['missing jobId', { ...wellFormed, fanOut: [{ payload: 'alpha', output: 'ALPHA#518' }] }],
    ['missing output', { ...wellFormed, fanOut: [{ ...wellFormed.fanOut[0]!, output: undefined }] }],
    ['NaN jobsDone', { ...wellFormed, workerCounts: [{ ...wellFormed.workerCounts[0]!, jobsDone: Number.NaN }] }],
    ['missing workersStopped', { ...wellFormed, cancellation: { ...wellFormed.cancellation, workersStopped: undefined } }],
    ['empty outcome', { ...wellFormed, cancellation: { ...wellFormed.cancellation, outcome: '' } }],
    ['non-object timeout beat', { ...wellFormed, timeouts: ['fast'] }],
    ['missing winner', { ...wellFormed, timeouts: [{ ...wellFormed.timeouts[0]!, winner: undefined }] }],
    ['missing received (absent is not empty)', { ...wellFormed, timeouts: [{ ...wellFormed.timeouts[1]!, received: undefined }] }],
    ['missing gomaxprocs', { ...wellFormed, runtime: { ...wellFormed.runtime, gomaxprocs: undefined } }],
    ['non-object setup', { ...wellFormed, setup: 8 }],
  ])('throws for %s', (_label, body) => {
    expect(() => concurrencyDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = concurrencyDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      runtime: { ...wellFormed.runtime, sneaky: true },
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toConcurrencyDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toConcurrencyDemo(wellFormed)).toBe(wellFormed);
  });
});
