import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ScalingStatelessApi } from './scaling-stateless-api';
import { ScalingStatelessRepositoryImpl } from './scaling-stateless-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ScalingStatelessRepositoryImpl spec — fixture-driven (happy +
 * failure): the statelessness round (config, distribution, steps,
 * counter, verdicts) round-trips; malformed payloads and transport
 * failures resolve as Failed Results, never throws.
 */

const demoPayload = {
  mode: 'one fleet, two placements',
  config: {
    replicas: 3,
    distributionRequests: 12,
    clientId: 'ada',
    counterIncrements: 6,
    localToken: 's-local-ada',
    sharedToken: 's-shared-ada',
    sharedStoreNote: 'an in-process analogue of the redis container',
  },
  distribution: {
    totalServed: 12,
    rows: [
      { replicaId: 'replica-1', served: 4 },
      { replicaId: 'replica-2', served: 4 },
      { replicaId: 'replica-3', served: 4 },
    ],
    note: '12 request(s) across 3 replica(s), per-replica 4/4/4',
  },
  steps: [
    {
      step: 1,
      phase: 'local-memory',
      label: 'login',
      route: 'round-robin',
      replicaId: 'replica-1',
      outcome: 'stored',
      detail: 'its own map, nobody else’s',
      observed: 'replica-1 holds 1 session(s)',
    },
  ],
  counter: {
    rows: [
      { replicaId: 'replica-1', increments: 2 },
      { replicaId: 'replica-2', increments: 2 },
      { replicaId: 'replica-3', increments: 2 },
    ],
    total: 6,
    note: 'read back as 6 from one store',
  },
  verdicts: [
    { id: 'distribution', label: 'the balancer spreads the work', value: '12 requests landed 4/4/4' },
  ],
  honesty: 'the round is deterministic',
};

describe('ScalingStatelessRepositoryImpl', () => {
  it('resolves the parsed payload as a Success', async () => {
    stubApiFetch([{ url: '/api/demo/scaling/stateless', body: demoPayload }]);
    const repository = new ScalingStatelessRepositoryImpl(
      new ScalingStatelessApi(createAppConfig()),
    );

    const result = await repository.get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.distribution.rows[0]?.served).toBe(4);
      expect(result.value.steps[0]?.outcome).toBe('stored');
      expect(result.value.counter.total).toBe(6);
    }
  });

  it('resolves a malformed payload as a Failure, never throws', async () => {
    stubApiFetch([{ url: '/api/demo/scaling/stateless', body: { steps: [] } }]);
    const repository = new ScalingStatelessRepositoryImpl(
      new ScalingStatelessApi(createAppConfig()),
    );

    const result = await repository.get();

    expect(result.ok).toBe(false);
  });

  it('resolves a transport failure as a Failure', async () => {
    stubNetworkFailure();
    const repository = new ScalingStatelessRepositoryImpl(
      new ScalingStatelessApi(createAppConfig()),
    );

    const result = await repository.get();

    expect(result.ok).toBe(false);
  });
});
