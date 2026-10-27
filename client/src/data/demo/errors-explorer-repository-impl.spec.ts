import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ErrorsExplorerApi } from './errors-explorer-api';
import { ErrorsExplorerRepositoryImpl } from './errors-explorer-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ErrorsExplorerRepositoryImpl spec — fixture-driven (happy + failure):
 * the explorer payload (kinds table, panels, live walk) round-trips;
 * malformed payloads and transport failures resolve as Failed Results,
 * never throws.
 */

const demoPayload = {
  kinds: [
    {
      kind: 'not_found',
      httpStatus: 404,
      defaultMessage: 'Resource not found.',
      raisedBy: 'use cases and handlers raise it directly',
      loggedAs: 'error response · warn',
      trigger: {
        mode: 'live',
        method: 'GET',
        path: '/api/posts/no-such-post',
        note: 'a well-formed id that names nothing',
      },
    },
  ],
  envelopeNote: 'one envelope for every failure',
  scenario: {
    getEndpoint: '/api/demo/scenario',
    patchEndpoint: '/api/demo/scenario',
    modes: ['none', 'error', 'latency'],
    note: 'the runtime toggle every demo reads',
  },
  probe: {
    probeEndpoint: '/api/demo/errors/probe',
    latencyMillis: 1500,
    healthyNote: 'answers 200 healthy',
    injectedNote: 'answers the recovered 500 envelope',
    latencyNote: 'sleeps out its 1500 ms injection',
  },
  walk: {
    injectFailure: 'none',
    status: 200,
    elapsedMillis: 2,
    requestID: 'rid-explorer-2',
    body: '{"status":"healthy","injectFailure":"none","detail":"probe answered normally"}',
    note: 'nothing injected: the probe answered 200 healthy in 2 ms',
  },
  reasoning: 'the panel never fakes an error it did not really produce',
};

function makeRepository(): ErrorsExplorerRepositoryImpl {
  return new ErrorsExplorerRepositoryImpl(new ErrorsExplorerApi(createAppConfig()));
}

describe('ErrorsExplorerRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/errors/explorer', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.kinds[0]?.httpStatus).toBe(404);
      expect(result.value.scenario.modes).toHaveLength(3);
      expect(result.value.walk.status).toBe(200);
      expect(result.value.reasoning).toContain('never fakes');
    }
  });

  it('rejects a payload whose walk row is missing', async () => {
    stubApiFetch([{ url: '/api/demo/errors/explorer', body: { ...demoPayload, walk: undefined } }]);

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
