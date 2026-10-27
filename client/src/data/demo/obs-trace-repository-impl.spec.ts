import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ObsTraceApi } from './obs-trace-api';
import { ObsTraceRepositoryImpl } from './obs-trace-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ObsTraceRepositoryImpl spec — fixture-driven (happy + failure): the
 * trace payload (acceptance panel, span tree, malformed table)
 * round-trips; malformed payloads and transport failures resolve as
 * Failed Results, never throws.
 */

const demoPayload = {
  accepted: {
    inbound: '',
    inboundOrigin: 'minted by the server for this walk',
    version: '00',
    traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
    parentId: '00f067aa0ba902b7',
    flags: '01',
    sampled: true,
    responseEcho: '00-4bf92f3577b34da6a3ce929d0e0e4736-96a123b077498b49-01',
    note: 'the echo carries the server span',
  },
  tree: [
    {
      name: 'GET /api/posts (traced walk)',
      layer: 'middleware',
      spanId: '96a123b077498b49',
      parentId: '00f067aa0ba902b7',
      traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
      depth: 0,
      startOffsetMs: 0,
      durationMs: 1.2,
      note: 'the graft point',
    },
  ],
  malformed: [
    {
      header: 'ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01',
      problem: 'traceparent …: version ff is reserved',
      handledAs: 'rejected; fresh root',
    },
  ],
  sdk: 'the SDK adds samplers and exporters',
  reasoning: 'one header became one tree',
};

function makeRepository(): ObsTraceRepositoryImpl {
  return new ObsTraceRepositoryImpl(new ObsTraceApi(createAppConfig()));
}

describe('ObsTraceRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/obs/trace', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.accepted.traceId).toBe('4bf92f3577b34da6a3ce929d0e0e4736');
      expect(result.value.tree[0]?.parentId).toBe('00f067aa0ba902b7');
      expect(result.value.malformed[0]?.problem).toContain('reserved');
    }
  });

  it('rejects a payload whose tree is missing', async () => {
    stubApiFetch([{ url: '/api/demo/obs/trace', body: { ...demoPayload, tree: undefined } }]);

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
