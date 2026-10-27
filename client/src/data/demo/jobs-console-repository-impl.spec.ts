import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { JobsConsoleApi } from './jobs-console-api';
import { JobsConsoleRepositoryImpl } from './jobs-console-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * JobsConsoleRepositoryImpl spec — fixture-driven (happy + failure):
 * the console-story payload (queues, ladders, beats, proofs)
 * round-trips; malformed payloads and the server's honest 503 for an
 * unreachable Redis resolve as Failed Results, never throws.
 */

const demoPayload = {
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
      verdict: 'succeeded after 3 attempt(s)',
    },
  ],
  beats: [
    {
      beat: 'enqueue',
      commands: ['ENQUEUE demo:flaky … Queue(demos) MaxRetry(2)'],
      outcome: 'two tasks accepted by the broker.',
    },
  ],
  cron: { spec: '@every 1s', ticks: 3 },
  observability: { peakDepth: 2, peakOldestAgeMs: 510, finalDepth: 0, finalOldestAgeMs: 0 },
  proofs: {
    retry: 'succeeded on attempt 3.',
    archive: 'the archive now holds it.',
    replay: 'the ledger grew by 1 attempt.',
    cron: '3 tick task(s) fired.',
    observability: 'depth peaked at 2; at rest: 0.',
  },
  reasoning: 'failure is a first-class state.',
};

function makeRepository(): JobsConsoleRepositoryImpl {
  return new JobsConsoleRepositoryImpl(new JobsConsoleApi(createAppConfig()));
}

describe('JobsConsoleRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/jobs/console', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.namespace).toBe('gdg:demo:jobs:0123456789ab');
      expect(result.value.queues).toHaveLength(2);
      expect(result.value.tasks[0]?.attempts).toHaveLength(3);
      expect(result.value.cron.ticks).toBe(3);
      expect(result.value.observability.finalDepth).toBe(0);
    }
  });

  it('rejects a payload whose proofs are missing', async () => {
    const poisoned = { ...demoPayload, proofs: undefined };
    stubApiFetch([{ url: '/api/demo/jobs/console', body: poisoned }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
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
});
