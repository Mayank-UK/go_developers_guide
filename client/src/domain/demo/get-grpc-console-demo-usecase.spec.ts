import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { GRPCConsoleDemoData } from './grpc-console';
import type { GRPCConsoleRepository } from './grpc-console-repository';
import { GetGRPCConsoleDemoUseCase } from './get-grpc-console-demo-usecase';

/**
 * GetGRPCConsoleDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<GRPCConsoleDemoData>): GRPCConsoleRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: GRPCConsoleDemoData = {
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
      summary: 'SyncTags took three updates and acked each one on the same call',
      statusCode: 'OK',
      statusNumber: 0,
      roundTrips: 1,
      results: [{ label: 'ack · p-7', detail: 'applied — tags now: go, rpc, streaming', ok: true }],
    },
  ],
};

describe('GetGRPCConsoleDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetGRPCConsoleDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetGRPCConsoleDemoUseCase(
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
