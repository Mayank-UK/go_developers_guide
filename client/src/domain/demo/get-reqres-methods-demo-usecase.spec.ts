import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { ReqresMethodsDemoData } from './reqres-methods';
import type { ReqresMethodsRepository } from './reqres-methods-repository';
import { GetReqresMethodsDemoUseCase } from './get-reqres-methods-demo-usecase';

/**
 * GetReqresMethodsDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<ReqresMethodsDemoData>): ReqresMethodsRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

/** Mirrors the Go builder's real output (trimmed to two representative rows). */
const data: ReqresMethodsDemoData = {
  resource: '/api/demo/reqres/methods/exhibit',
  allow: 'GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE',
  methods: [
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
      method: 'PATCH',
      status: 204,
      statusText: 'No Content',
      headers: [{ name: 'X-Reqres-Method', value: 'PATCH' }],
      bodyShape: 'no body — 204 means the headers are the whole answer',
      body: '',
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

describe('GetReqresMethodsDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetReqresMethodsDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetReqresMethodsDemoUseCase(
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
