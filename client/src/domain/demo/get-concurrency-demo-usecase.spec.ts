import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { ConcurrencyDemoData } from './concurrency';
import type { ConcurrencyRepository } from './concurrency-repository';
import { GetConcurrencyDemoUseCase } from './get-concurrency-demo-usecase';

/**
 * GetConcurrencyDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<ConcurrencyDemoData>): ConcurrencyRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

/** Mirrors the real builder output (deterministic beats; one observed row each). */
const data: ConcurrencyDemoData = {
  setup: { jobs: 8, workers: 3, cancelAfter: 5, strategy: 'worker pool: fan-out, fan-in, cancellation via select' },
  fanOut: [
    { jobId: 'job-01', payload: 'alpha', output: 'ALPHA#518' },
    { jobId: 'job-02', payload: 'bravo', output: 'BRAVO#538' },
  ],
  workerCounts: [
    { workerId: 'worker-1', jobsDone: 3 },
    { workerId: 'worker-2', jobsDone: 2 },
    { workerId: 'worker-3', jobsDone: 3 },
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

describe('GetConcurrencyDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetConcurrencyDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetConcurrencyDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
