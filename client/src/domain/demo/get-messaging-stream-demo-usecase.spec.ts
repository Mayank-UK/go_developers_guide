import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { MessagingStreamDemoData } from './messaging-stream';
import type { MessagingStreamRepository } from './messaging-stream-repository';
import { GetMessagingStreamDemoUseCase } from './get-messaging-stream-demo-usecase';

/**
 * GetMessagingStreamDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

function fakeRepository(get: Result<MessagingStreamDemoData>): MessagingStreamRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: MessagingStreamDemoData = {
  namespace: 'gdg:demo:msg:0123456789ab',
  stream: 'gdg:demo:msg:0123456789ab:orders',
  group: 'workers',
  commands: ['XADD', 'XREADGROUP', 'XACK', 'XPENDING', 'XAUTOCLAIM'],
  beats: [
    {
      beat: 'append',
      commands: ['XADD gdg:demo:msg:0123456789ab:orders … step created'],
      outcome: 'three entries appended.',
      entries: ['1780000000000-0', '1780000000001-0', '1780000000002-0'],
    },
  ],
  proofs: {
    ordering: 'consumer-a answered the same ids in the same order.',
    queueProof: 'consumer-b answered 0 entries.',
    recovery: 'XAUTOCLAIM moved exactly the stalled entry.',
    pendingLeft: 0,
  },
  reasoning: 'a stream is a log; a group is a ledger.',
};

describe('GetMessagingStreamDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetMessagingStreamDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetMessagingStreamDemoUseCase(
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
