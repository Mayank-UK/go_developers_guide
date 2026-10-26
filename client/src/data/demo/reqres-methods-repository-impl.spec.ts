import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ReqresMethodsApi } from './reqres-methods-api';
import { ReqresMethodsRepositoryImpl } from './reqres-methods-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ReqresMethodsRepositoryImpl spec — fixture-driven (happy + failure):
 * the happy payload round-trips into the domain shape; network /
 * envelope / malformed-payload failures all resolve as Failed Results,
 * never throws. The happy fixture mirrors the Go builder's real output
 * (trimmed to three representative method rows).
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  resource: '/api/demo/reqres/methods/exhibit',
  allow: 'GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE',
  methods: [
    {
      method: 'GET',
      status: 200,
      statusText: 'OK',
      headers: [
        { name: 'Content-Type', value: 'application/json' },
        { name: 'Cache-Control', value: 'no-store' },
        { name: 'Set-Cookie', value: 'reqres_visit=1; Path=/api/demo/reqres; HttpOnly; SameSite=Lax' },
        { name: 'X-Reqres-Method', value: 'GET' },
      ],
      bodyShape: "one JSON object — the resource's current state",
      body: '{"resource":"methods-exhibit","state":"steady"}',
    },
    {
      method: 'POST',
      status: 201,
      statusText: 'Created',
      headers: [
        { name: 'Content-Type', value: 'application/json' },
        { name: 'Location', value: '/api/demo/reqres/methods/exhibit/17' },
        { name: 'X-Reqres-Method', value: 'POST' },
      ],
      bodyShape: 'one JSON object — the created record; Location names where it lives',
      body: '{"id":17,"state":"created"}',
    },
    {
      method: 'TRACE',
      status: 405,
      statusText: 'Method Not Allowed',
      headers: [
        { name: 'Allow', value: 'GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE' },
        { name: 'X-Reqres-Method', value: 'TRACE' },
      ],
      bodyShape: 'one JSON object — the error note; Allow states what IS served',
      body: '{"error":"TRACE","served":"GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE"}',
    },
  ],
  sse: {
    contentType: 'text/event-stream',
    events: [
      { name: 'beat', data: '{"sequence":1,"state":"tick"}' },
      { name: 'beat', data: '{"sequence":2,"state":"tick"}' },
      { name: 'beat', data: '{"sequence":3,"state":"tick"}' },
    ],
    flushes: 3,
    framing: 'each event block ends with a blank line — the delimiter a client parser waits for',
  },
};

function makeRepository(): ReqresMethodsRepositoryImpl {
  return new ReqresMethodsRepositoryImpl(new ReqresMethodsApi(createAppConfig()));
}

describe('ReqresMethodsRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/reqres/methods', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.resource).toBe('/api/demo/reqres/methods/exhibit');
      expect(result.value.methods[1]?.status).toBe(201);
      expect(result.value.methods[1]?.headers[1]?.name).toBe('Location');
      expect(result.value.methods[2]?.status).toBe(405);
      expect(result.value.sse.events).toHaveLength(3);
      expect(result.value.sse.flushes).toBe(3);
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
        url: '/api/demo/reqres/methods',
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
      // Missing the method rows — the DTO validator must reject it.
      { url: '/api/demo/reqres/methods', body: { resource: demoPayload.resource } },
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

  it('rejects a non-HTTP status instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/reqres/methods',
        body: {
          ...demoPayload,
          methods: demoPayload.methods.map((row) => ({ ...row, status: 999 })),
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
