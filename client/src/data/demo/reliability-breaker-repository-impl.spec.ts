import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ReliabilityBreakerApi } from './reliability-breaker-api';
import { ReliabilityBreakerRepositoryImpl } from './reliability-breaker-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ReliabilityBreakerRepositoryImpl spec — fixture-driven (happy +
 * failure): the breaker payload (scenario, persistent breaker, walk,
 * retry) round-trips; malformed payloads and transport failures
 * resolve as Failed Results, never throws.
 */

const demoPayload = {
  mode: 'process-lifetime breaker: one shared circuit breaker',
  scenario: {
    mode: 'none',
    getEndpoint: '/api/demo/scenario',
    patchEndpoint: '/api/demo/scenario',
    modes: ['none', 'error', 'latency'],
    note: 'the runtime toggle the dependency honors',
  },
  breaker: {
    state: 'closed',
    stateAgeMs: 45000,
    config: {
      failureThreshold: 3,
      cooldownMs: 2000,
      callTimeoutMs: 120,
      walkAttempts: 4,
      halfOpenProbes: 1,
      retryAttempts: 3,
      retryBaseDelayMs: 40,
      retryJitterMs: 20,
    },
    counters: { allowed: 4, fastFailed: 0, succeeded: 4, failed: 0 },
    probeInFlight: false,
    transitions: [],
  },
  walk: {
    mode: 'none',
    rows: [
      { index: 1, stateBefore: 'closed', action: 'call', outcome: 'success', elapsedMs: 0.03, stateAfter: 'closed' },
    ],
    note: 'every one of the 1 attempts reached the dependency and succeeded',
  },
  retry: {
    mode: 'none',
    succeeded: true,
    rows: [{ attempt: 1, outcome: 'success', elapsedMs: 0.02, backoffMs: 0 }],
    totalElapsedMs: 0.05,
    note: 'the series succeeded on attempt 1',
  },
  reasoning: 'the breaker state persists across requests',
};

describe('ReliabilityBreakerRepositoryImpl', () => {
  it('resolves the parsed payload as a Success', async () => {
    stubApiFetch([{ url: '/api/demo/reliability/breaker', body: demoPayload }]);
    const repository = new ReliabilityBreakerRepositoryImpl(
      new ReliabilityBreakerApi(createAppConfig()),
    );

    const result = await repository.get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.breaker.state).toBe('closed');
      expect(result.value.walk.rows[0]?.outcome).toBe('success');
    }
  });

  it('resolves a malformed payload as a Failure, never throws', async () => {
    stubApiFetch([{ url: '/api/demo/reliability/breaker', body: { breaker: { state: 'closed' } } }]);
    const repository = new ReliabilityBreakerRepositoryImpl(
      new ReliabilityBreakerApi(createAppConfig()),
    );

    const result = await repository.get();

    expect(result.ok).toBe(false);
  });

  it('resolves a transport failure as a Failure', async () => {
    stubNetworkFailure();
    const repository = new ReliabilityBreakerRepositoryImpl(
      new ReliabilityBreakerApi(createAppConfig()),
    );

    const result = await repository.get();

    expect(result.ok).toBe(false);
  });
});
