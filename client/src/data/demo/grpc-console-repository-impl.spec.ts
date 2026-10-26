import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { GRPCConsoleApi } from './grpc-console-api';
import { GRPCConsoleRepositoryImpl } from './grpc-console-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * GRPCConsoleRepositoryImpl spec — fixture-driven (happy + failure): the
 * happy payload round-trips into the domain shape; network / envelope /
 * malformed-payload failures all resolve as Failed Results, never throws.
 * The happy fixture mirrors the Go builder's output (see the DTO spec for
 * the full value-for-value mirror); here a compact slice suffices because
 * the DTO spec owns the deep shape.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  transport: {
    mode: 'in-process bufconn listener — real HTTP/2 RPCs, no TCP port',
    dialTarget: 'passthrough:///bufnet',
    security: 'plaintext credentials, explicit — grpc refuses implicit insecure',
    protoFile: 'server/api/proto/posts.proto',
    service: 'guide.posts.v1.PostsService',
    generatedGo: 'server/internal/grpcapi/pb',
    codegenCmd: 'buf generate',
  },
  calls: [
    {
      kind: 'unary',
      method: '/guide.posts.v1.PostsService/GetPost',
      pattern: '1 request → 1 response',
      summary: 'GetPost("p-3") returned the full post — one message in, one message out',
      statusCode: 'OK',
      statusNumber: 0,
      roundTrips: 1,
      results: [
        { label: 'request', detail: 'PostRequest{Id: "p-3"}', ok: true },
        { label: 'response', detail: '"Errors are values" · tags: go, errors', ok: true },
      ],
    },
    {
      kind: 'bidi',
      method: '/guide.posts.v1.PostsService/SyncTags',
      pattern: 'a stream of requests ⇄ a stream of responses',
      summary:
        'SyncTags took three updates and acked each one on the same call — the unknown post drew a rejection ack, and the stream kept going',
      statusCode: 'OK',
      statusNumber: 0,
      roundTrips: 1,
      results: [
        { label: 'messages sent', detail: '3 TagUpdates on one call, then CloseSend', ok: true },
        {
          label: 'ack · p-999',
          detail: 'rejected — Post "p-999" not found. — acked per item; the stream continues',
          ok: false,
        },
      ],
    },
  ],
};

function makeRepository(): GRPCConsoleRepositoryImpl {
  return new GRPCConsoleRepositoryImpl(new GRPCConsoleApi(createAppConfig()));
}

describe('GRPCConsoleRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/api/grpc-console', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.transport.service).toBe('guide.posts.v1.PostsService');
      expect(result.value.calls).toHaveLength(2);
      expect(result.value.calls[0]?.statusCode).toBe('OK');
      expect(result.value.calls[1]?.results[1]?.ok).toBe(false);
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
        url: '/api/demo/api/grpc-console',
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
      // Missing the exhibit calls — the DTO validator must reject it.
      { url: '/api/demo/api/grpc-console', body: { transport: demoPayload.transport } },
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

  it('rejects an unknown method kind instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/api/grpc-console',
        body: {
          ...demoPayload,
          calls: [{ ...demoPayload.calls[0]!, kind: 'trinary' }],
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
