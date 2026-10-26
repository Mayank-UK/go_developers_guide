import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { REDACTED_HEADER_VALUE } from '../../domain/demo/http-headers';
import { HttpHeadersApi } from './http-headers-api';
import { HttpHeadersRepositoryImpl } from './http-headers-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * HttpHeadersRepositoryImpl spec — fixture-driven (happy + failure): the
 * happy payload round-trips into the domain shape; network / envelope /
 * malformed-payload failures all resolve as Failed Results, never throws.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  method: 'GET',
  path: '/api/demo/http/headers',
  query: 'verbose=1',
  proto: 'HTTP/1.1',
  requestID: 'req-17',
  headers: [
    {
      name: 'User-Agent',
      value: 'curl/8.7.1',
      scope: 'end-to-end',
      role: 'identity',
      note: 'identifies the calling software; log it, rarely branch on it',
    },
    { name: 'Accept', value: 'application/json', scope: 'end-to-end', role: 'negotiation' },
    { name: 'Authorization', value: REDACTED_HEADER_VALUE, scope: 'end-to-end', role: 'security' },
    { name: 'X-Custom', value: 'teaching-value', scope: 'end-to-end', role: 'extension' },
    { name: 'Connection', value: 'keep-alive', scope: 'hop-by-hop', role: 'transport' },
  ],
  hopByHop: [
    'Connection',
    'Keep-Alive',
    'Proxy-Authenticate',
    'Proxy-Authorization',
    'Te',
    'Trailer',
    'Transfer-Encoding',
    'Upgrade',
  ],
};

function makeRepository(): HttpHeadersRepositoryImpl {
  return new HttpHeadersRepositoryImpl(new HttpHeadersApi(createAppConfig()));
}

describe('HttpHeadersRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/http/headers', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.method).toBe('GET');
      expect(result.value.requestID).toBe('req-17');
      expect(result.value.headers[2]?.value).toBe(REDACTED_HEADER_VALUE);
      expect(result.value.headers[4]?.scope).toBe('hop-by-hop');
      expect(result.value.hopByHop).toHaveLength(8);
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
        url: '/api/demo/http/headers',
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
      // Missing the echoed headers — the DTO validator must reject it.
      { url: '/api/demo/http/headers', body: { method: 'GET', path: '/' } },
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

  it('rejects an unknown role vocabulary instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/http/headers',
        body: {
          ...demoPayload,
          headers: [{ name: 'Accept', value: '*/*', scope: 'end-to-end', role: 'mystery' }],
        },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });
});
