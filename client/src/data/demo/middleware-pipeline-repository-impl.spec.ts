import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { MiddlewarePipelineApi } from './middleware-pipeline-api';
import { MiddlewarePipelineRepositoryImpl } from './middleware-pipeline-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * MiddlewarePipelineRepositoryImpl spec — fixture-driven (happy +
 * failure): the happy payload (the REAL Go builder output, abbreviated
 * here to its load-bearing rows) round-trips into the domain shape;
 * network / envelope / malformed-payload failures all resolve as Failed
 * Results, never throws.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  chain: [
    { name: 'request-id', position: 0, role: 'assigns or honors one id', mirrors: 'presentation.RequestID' },
    { name: 'request-logger', position: 1, role: 'one line per request', mirrors: 'presentation.RequestLogger' },
    { name: 'recover', position: 2, role: 'panics to 500 envelopes', mirrors: 'presentation.Recover' },
    { name: 'terminal handler', position: 3, role: 'the endpoint', mirrors: 'http.ServeMux' },
  ],
  trace: [
    { layer: 'request-id', phase: 'before', sequence: 0, requestIDSeen: '', note: 'not yet assigned' },
    { layer: 'terminal handler', phase: 'handler', sequence: 3, requestIDSeen: 'middleware-demo-42', note: 'ran once' },
    { layer: 'request-id', phase: 'after', sequence: 6, requestIDSeen: 'middleware-demo-42', note: 'header stamped early' },
  ],
  requestSideOrder: ['request-id', 'request-logger', 'recover', 'terminal handler'],
  responseSideOrder: ['terminal handler', 'recover', 'request-logger', 'request-id'],
  requestID: {
    headerName: 'x-request-id',
    inbound: 'middleware-demo-42',
    seenAtLayers: ['terminal handler (handler)', 'request-id (after)'],
    responseHeader: 'middleware-demo-42',
    sameIDEverywhere: true,
  },
  panicRecovery: {
    path: '/api/demo/middleware/pipeline',
    cause: 'assignment to entry in nil map',
    status: 500,
    body: '{"error":{"code":"internal"}}\n',
    loggedLine: 'level=ERROR msg="panic recovered" request_id=middleware-demo-42',
    requestIDKept: true,
    trace: [],
  },
  timeoutBeat: {
    wrapper: 'http.TimeoutHandler',
    budgetMs: 30,
    handlerSleepMs: 120,
    observedStatus: 503,
    body: '<html><h1>Timeout</h1></html>',
    slowHandlerFinished: true,
    shape: 'the wrapper answers 503 itself',
  },
  reasoning: 'Wrap order is request order.',
};

function makeRepository(): MiddlewarePipelineRepositoryImpl {
  return new MiddlewarePipelineRepositoryImpl(new MiddlewarePipelineApi(createAppConfig()));
}

describe('MiddlewarePipelineRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/middleware/pipeline', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.chain).toHaveLength(4);
      expect(result.value.chain[0]?.mirrors).toBe('presentation.RequestID');
      expect(result.value.trace[0]?.requestIDSeen).toBe('');
      expect(result.value.requestID.sameIDEverywhere).toBe(true);
      expect(result.value.panicRecovery.status).toBe(500);
      expect(result.value.timeoutBeat.observedStatus).toBe(503);
      expect(result.value.timeoutBeat.slowHandlerFinished).toBe(true);
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
        url: '/api/demo/middleware/pipeline',
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
      // Missing the traced run — the DTO validator must reject it.
      { url: '/api/demo/middleware/pipeline', body: { chain: [] } },
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

  it('rejects an unknown trace phase instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/middleware/pipeline',
        body: {
          ...demoPayload,
          trace: [{ layer: 'request-id', phase: 'sideways', sequence: 0, requestIDSeen: '', note: 'nope' }],
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
