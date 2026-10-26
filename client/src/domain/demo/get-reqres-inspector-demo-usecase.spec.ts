import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { ReqresInspectorDemoData } from './reqres-inspector';
import type { ReqresInspectorRepository } from './reqres-inspector-repository';
import { GetReqresInspectorDemoUseCase } from './get-reqres-inspector-demo-usecase';

/**
 * GetReqresInspectorDemoUseCase spec — the use case is a thin
 * pass-through seam, so the contract to pin is exactly that: whatever
 * the repository resolves (Success or Failed) arrives unchanged, and
 * nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<ReqresInspectorDemoData>): ReqresInspectorRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

/** Mirrors the Go builder's real output for a body-less GET. */
const data: ReqresInspectorDemoData = {
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
  ],
  liveBody: { sent: false, outcome: 'none' },
};

describe('GetReqresInspectorDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetReqresInspectorDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetReqresInspectorDemoUseCase(
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
