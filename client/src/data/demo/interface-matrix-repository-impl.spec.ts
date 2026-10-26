import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { InterfaceMatrixApi } from './interface-matrix-api';
import { InterfaceMatrixRepositoryImpl } from './interface-matrix-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * InterfaceMatrixRepositoryImpl spec — fixture-driven (happy + failure):
 * the happy payload round-trips into the domain shape; network / envelope /
 * malformed-payload failures all resolve as Failed Results, never throws.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  interfaces: [
    { name: 'io.Reader', method: 'Read([]byte) (int, error)', pkg: 'io', note: 'The canon.' },
    { name: 'error', method: 'Error() string', pkg: 'builtin', note: 'The language\u2019s own interface.' },
  ],
  rows: [
    {
      expression: '*core.Error',
      type: '*core.Error',
      kind: 'ptr',
      methods: ['Error', 'Is', 'Unwrap'],
      cells: [
        { iface: 'io.Reader', satisfied: false },
        { iface: 'error', satisfied: true },
      ],
    },
  ],
  embeddings: [
    {
      expression: 'type delegatingHandler struct { http.Handler }',
      embeds: 'http.Handler (interface)',
      type: 'demos.delegatingHandler',
      iface: 'http.Handler',
      satisfied: true,
      note: 'An embedded interface promotes its whole method set.',
    },
  ],
  typeSwitch: [
    { input: 'time.Duration', matchedCase: 'case fmt.Stringer', result: 'fmt.Stringer case — 1m30s' },
  ],
  compileProofs: [{ line: '_ http.Handler  = http.HandlerFunc(nil)', pins: 'http.Handler × http.HandlerFunc' }],
};

function makeRepository(): InterfaceMatrixRepositoryImpl {
  return new InterfaceMatrixRepositoryImpl(new InterfaceMatrixApi(createAppConfig()));
}

describe('InterfaceMatrixRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/iface/matrix', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.interfaces[0]?.method).toBe('Read([]byte) (int, error)');
      expect(result.value.rows[0]?.cells[1]).toEqual({ iface: 'error', satisfied: true });
      expect(result.value.embeddings[0]?.satisfied).toBe(true);
      expect(result.value.compileProofs).toHaveLength(1);
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
        url: '/api/demo/iface/matrix',
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
      // Missing the matrix rows — the DTO validator must reject it.
      { url: '/api/demo/iface/matrix', body: { interfaces: demoPayload.interfaces } },
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
});
