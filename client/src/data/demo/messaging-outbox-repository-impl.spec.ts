import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { MessagingOutboxApi } from './messaging-outbox-api';
import { MessagingOutboxRepositoryImpl } from './messaging-outbox-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * MessagingOutboxRepositoryImpl spec — fixture-driven (happy + failure):
 * the outbox-story payload (beats, proofs) round-trips; malformed
 * payloads and the server's honest 503 for unreachable stores resolve
 * as Failed Results, never throws.
 */

const demoPayload = {
  namespace: 'gdg:demo:msg:0123456789ab',
  stream: 'gdg:demo:msg:0123456789ab:outbox',
  beats: [
    {
      beat: 'relay',
      commands: [
        'SELECT … WHERE published_at IS NULL FOR UPDATE SKIP LOCKED',
        'XADD gdg:demo:msg:0123456789ab:outbox messageId … payload …',
        'UPDATE msgdemo_outbox SET published_at = now()',
        'COMMIT',
      ],
      outcome: 'the relay published 2 message(s); a second pass drained 0.',
      ids: ['m-aaaaaaaaaaaaaaaa', 'm-bbbbbbbbbbbbbbbb'],
    },
    {
      beat: 'redeliver',
      commands: ['XAUTOCLAIM gdg:demo:msg:0123456789ab:outbox workers consumer-b 0 0-0 COUNT 10'],
      outcome: 'the stalled entry re-delivered to consumer-b was refused by dedup 1 time(s).',
      ids: ['1780000000000-0'],
    },
  ],
  proofs: {
    atomicity: 'the count query answered 0 — all-or-nothing is not a hope.',
    relayProof: 'a second pass drained 0.',
    idempotency: 'the redelivery was refused 1 time(s).',
    duplicates: 1,
    eventsDeleted: 2,
    outboxDeleted: 2,
  },
  reasoning: 'exactly-once EFFECT is one SET NX away.',
};

function makeRepository(): MessagingOutboxRepositoryImpl {
  return new MessagingOutboxRepositoryImpl(new MessagingOutboxApi(createAppConfig()));
}

describe('MessagingOutboxRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/msg/outbox', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.namespace).toBe('gdg:demo:msg:0123456789ab');
      expect(result.value.beats[0]?.ids).toHaveLength(2);
      expect(result.value.proofs.duplicates).toBe(1);
      expect(result.value.proofs.eventsDeleted).toBe(2);
    }
  });

  it('rejects a payload whose beats carry an empty command', async () => {
    const poisoned = {
      ...demoPayload,
      beats: [{ ...demoPayload.beats[0], commands: ['BEGIN', ''] }],
    };
    stubApiFetch([{ url: '/api/demo/msg/outbox', body: poisoned }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
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
});
