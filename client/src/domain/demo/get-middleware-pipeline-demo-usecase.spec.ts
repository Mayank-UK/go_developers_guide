import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { MiddlewarePipelineDemoData } from './middleware-pipeline';
import type { MiddlewarePipelineRepository } from './middleware-pipeline-repository';
import { GetMiddlewarePipelineDemoUseCase } from './get-middleware-pipeline-demo-usecase';

/**
 * GetMiddlewarePipelineDemoUseCase spec — the use case is a thin
 * pass-through seam, so the contract to pin is exactly that: whatever the
 * repository resolves (Success or Failed) arrives unchanged, and nothing
 * throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<MiddlewarePipelineDemoData>): MiddlewarePipelineRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: MiddlewarePipelineDemoData = {
  chain: [
    { name: 'request-id', position: 0, role: 'assigns or honors one id', mirrors: 'presentation.RequestID' },
    { name: 'terminal handler', position: 3, role: 'the endpoint', mirrors: 'http.ServeMux' },
  ],
  trace: [
    { layer: 'request-id', phase: 'before', sequence: 0, requestIDSeen: '', note: 'not yet assigned' },
    { layer: 'terminal handler', phase: 'handler', sequence: 3, requestIDSeen: 'middleware-demo-42', note: 'ran once' },
  ],
  requestSideOrder: ['request-id', 'terminal handler'],
  responseSideOrder: ['terminal handler', 'request-id'],
  requestID: {
    headerName: 'x-request-id',
    inbound: 'middleware-demo-42',
    seenAtLayers: ['terminal handler (handler)'],
    responseHeader: 'middleware-demo-42',
    sameIDEverywhere: true,
  },
  panicRecovery: {
    path: '/api/demo/middleware/pipeline',
    cause: 'assignment to entry in nil map',
    status: 500,
    body: '{"error":{"code":"internal"}}\n',
    loggedLine: 'level=ERROR msg="panic recovered"',
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

describe('GetMiddlewarePipelineDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetMiddlewarePipelineDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetMiddlewarePipelineDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
