import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ConcurrencyApi } from './concurrency-api';
import { ConcurrencyRepositoryImpl } from './concurrency-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ConcurrencyRepositoryImpl spec — fixture-driven (happy + failure): the
 * happy payload mirrors the real builder output and round-trips into the
 * domain shape; network / envelope / malformed-payload failures all
 * resolve as Failed Results, never throws.
 */

/** A minimal but structurally complete wire payload (real builder shape). */
const demoPayload = {
  setup: { jobs: 8, workers: 3, cancelAfter: 5, strategy: 'worker pool: fan-out over one jobs channel, fan-in over one results channel, cancellation via select' },
  fanOut: [
    { jobId: 'job-01', payload: 'alpha', output: 'ALPHA#518' },
    { jobId: 'job-02', payload: 'bravo', output: 'BRAVO#538' },
    { jobId: 'job-03', payload: 'charlie', output: 'CHARLIE#728' },
  ],
  workerCounts: [
    { workerId: 'worker-1', jobsDone: 2 },
    { workerId: 'worker-2', jobsDone: 1 },
    { workerId: 'worker-3', jobsDone: 0 },
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

function makeRepository(): ConcurrencyRepositoryImpl {
  return new ConcurrencyRepositoryImpl(new ConcurrencyApi(createAppConfig()));
}

describe('ConcurrencyRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/runtime/concurrency', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.setup.jobs).toBe(8);
      expect(result.value.fanOut[0]?.output).toBe('ALPHA#518');
      expect(result.value.cancellation.jobsAbandoned).toBe(3);
      // The timer-won race carries its empty received as data.
      expect(result.value.timeouts[1]?.received).toBe('');
      expect(result.value.runtime.gomaxprocs).toBe(8);
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });

  it('guards an error envelope into a Failed Result with its diagnostics', async () => {
    stubApiFetch([
      {
        url: '/api/demo/runtime/concurrency',
        status: 503,
        body: { error: { code: 'service_unavailable', message: 'calm down', requestId: 'req-9' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('calm down', {
          code: 'service_unavailable',
          status: 503,
          requestId: 'req-9',
        }),
      ),
    );
  });

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // Missing every beat — the DTO validator must reject it.
      { url: '/api/demo/runtime/concurrency', body: { fanOut: 'nope' } },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('The server response did not match the expected shape.', {
          code: 'invalid_response',
          status: 200,
        }),
      ),
    );
  });

  it('rejects a non-number jobs count instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/runtime/concurrency',
        body: { ...demoPayload, setup: { ...demoPayload.setup, jobs: 'eight' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });
});
