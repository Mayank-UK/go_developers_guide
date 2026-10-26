import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ReqresInspectorApi } from './reqres-inspector-api';
import { ReqresInspectorRepositoryImpl } from './reqres-inspector-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ReqresInspectorRepositoryImpl spec — fixture-driven (happy + failure):
 * the happy payload round-trips into the domain shape; network /
 * envelope / malformed-payload failures all resolve as Failed Results,
 * never throws. The happy fixture mirrors the Go builder's real output
 * for a body-less GET.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  request: {
    method: 'GET',
    path: '/api/demo/reqres/inspector',
    bodyBytes: 0,
    bodyLimit: 4096,
  },
  discipline: {
    decoder: 'json.NewDecoder over the request body',
    unknownFields: 'rejected — DisallowUnknownFields',
    trailing: 'exactly one JSON value — a second Decode must return io.EOF',
    firstError: 'reported verbatim; decoding stops at the first offense',
  },
  exhibits: [
    {
      label: 'valid body',
      body: '{"title":"Read the body as a stream","tags":["request","json"]}',
      outcome: 'accepted',
      decoded: 'title="Read the body as a stream" tags=["request" "json"]',
    },
    {
      label: 'unknown field, then a type error — first error wins',
      body: '{"title":"Valid title","urgent":true,"tags":"not-a-list"}',
      outcome: 'rejected',
      error: 'json: unknown field "urgent"',
      errorType: '*errors.errorString',
      decoded: 'title="Valid title" tags=[]',
    },
    {
      label: 'truncated mid-array',
      body: '{"title":"complete","tags":["a"',
      outcome: 'rejected',
      error: 'unexpected EOF',
      errorType: '*errors.errorString',
      unexpectedEOF: true,
      decoded: 'title="" tags=[]',
    },
    {
      label: 'wrong-typed field',
      body: '{"title":42,"tags":[]}',
      outcome: 'rejected',
      error: 'json: cannot unmarshal number into Go struct field reqresBody.title of type string',
      errorType: '*json.UnmarshalTypeError',
      field: 'title',
      decoded: 'title="" tags=[]',
    },
  ],
  liveBody: { sent: false, outcome: 'none' },
};

function makeRepository(): ReqresInspectorRepositoryImpl {
  return new ReqresInspectorRepositoryImpl(new ReqresInspectorApi(createAppConfig()));
}

describe('ReqresInspectorRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/reqres/inspector', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.request.method).toBe('GET');
      expect(result.value.request.bodyLimit).toBe(4096);
      expect(result.value.exhibits).toHaveLength(4);
      expect(result.value.exhibits[2]?.unexpectedEOF).toBe(true);
      expect(result.value.exhibits[3]?.errorType).toBe('*json.UnmarshalTypeError');
      expect(result.value.liveBody.outcome).toBe('none');
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
        url: '/api/demo/reqres/inspector',
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
      // Missing the exhibits — the DTO validator must reject it.
      { url: '/api/demo/reqres/inspector', body: { request: demoPayload.request } },
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

  it('rejects an unknown outcome vocabulary instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/reqres/inspector',
        body: {
          ...demoPayload,
          exhibits: demoPayload.exhibits.map((exhibit) => ({ ...exhibit, outcome: 'maybe' })),
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
