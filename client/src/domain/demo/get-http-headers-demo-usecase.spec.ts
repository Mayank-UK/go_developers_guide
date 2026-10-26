import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { HttpHeadersDemoData } from './http-headers';
import type { HttpHeadersRepository } from './http-headers-repository';
import { GetHttpHeadersDemoUseCase } from './get-http-headers-demo-usecase';

/**
 * GetHttpHeadersDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<HttpHeadersDemoData>): HttpHeadersRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: HttpHeadersDemoData = {
  method: 'GET',
  path: '/api/demo/http/headers',
  query: 'verbose=1',
  proto: 'HTTP/1.1',
  requestID: 'req-17',
  headers: [
    { name: 'User-Agent', value: 'curl/8.7.1', scope: 'end-to-end', role: 'identity' },
    { name: 'Authorization', value: '[value withheld]', scope: 'end-to-end', role: 'security' },
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

describe('GetHttpHeadersDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetHttpHeadersDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetHttpHeadersDemoUseCase(
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
