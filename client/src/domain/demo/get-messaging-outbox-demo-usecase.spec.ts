import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { MessagingOutboxDemoData } from './messaging-outbox';
import type { MessagingOutboxRepository } from './messaging-outbox-repository';
import { GetMessagingOutboxDemoUseCase } from './get-messaging-outbox-demo-usecase';

/**
 * GetMessagingOutboxDemoUseCase spec — the use case is a thin
 * pass-through seam, so the contract to pin is exactly that: whatever
 * the repository resolves (Success or Failed) arrives unchanged, and
 * nothing throws.
 */

function fakeRepository(get: Result<MessagingOutboxDemoData>): MessagingOutboxRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: MessagingOutboxDemoData = {
  namespace: 'gdg:demo:msg:0123456789ab',
  stream: 'gdg:demo:msg:0123456789ab:outbox',
  beats: [
    {
      beat: 'record (commit)',
      commands: ['BEGIN', 'INSERT INTO msgdemo_events …', 'INSERT INTO msgdemo_outbox …', 'COMMIT'],
      outcome: 'two event rows and their outbox rows committed.',
      ids: ['m-aaaaaaaaaaaaaaaa', 'm-bbbbbbbbbbbbbbbb'],
    },
  ],
  proofs: {
    atomicity: 'the count query answered 0.',
    relayProof: 'a second pass drained 0.',
    idempotency: 'refused 1 time(s).',
    duplicates: 1,
    eventsDeleted: 2,
    outboxDeleted: 2,
  },
  reasoning: 'exactly-once EFFECT is one SET NX away.',
};

describe('GetMessagingOutboxDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetMessagingOutboxDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetMessagingOutboxDemoUseCase(
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
