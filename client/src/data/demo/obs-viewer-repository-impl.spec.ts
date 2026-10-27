import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ObsViewerApi } from './obs-viewer-api';
import { ObsViewerRepositoryImpl } from './obs-viewer-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ObsViewerRepositoryImpl spec — fixture-driven (happy + failure): the
 * viewer payload (anatomy, level matrix, correlated replay) round-
 * trips; malformed payloads and transport failures resolve as Failed
 * Results, never throws.
 */

const demoPayload = {
  anatomy: {
    line: '{"level":"INFO","msg":"http_request","status":200}',
    fields: [
      { key: 'level', value: '"INFO"', note: 'the dial the line passed' },
      { key: 'status', value: '200', note: 'the health of the request' },
    ],
    note: 'verbatim what slog wrote',
  },
  levels: {
    rows: [
      { level: 'INFO', survivesAt: ['debug', 'info'], emittedBy: 'the heartbeat', note: 'measured' },
    ],
    gate: 'GDG_LOG_LEVEL',
    note: 'asymmetric by design',
  },
  replay: {
    requests: [
      { label: 'the 404', request: 'GET /api/posts/x', requestId: 'rid-obs-9', status: 404 },
    ],
    lines: [
      { source: 'the 404 (its failure line)', level: 'WARN', msg: 'error response', requestId: 'rid-obs-9', raw: '{}' },
    ],
    note: 'one id threads them',
  },
  reasoning: 'the log is data',
};

function makeRepository(): ObsViewerRepositoryImpl {
  return new ObsViewerRepositoryImpl(new ObsViewerApi(createAppConfig()));
}

describe('ObsViewerRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/obs/viewer', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.anatomy.fields[0]?.key).toBe('level');
      expect(result.value.levels.rows[0]?.survivesAt).toEqual(['debug', 'info']);
      expect(result.value.replay.lines[0]?.requestId).toBe('rid-obs-9');
    }
  });

  it('rejects a payload whose anatomy panel is missing', async () => {
    stubApiFetch([{ url: '/api/demo/obs/viewer', body: { ...demoPayload, anatomy: undefined } }]);

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
