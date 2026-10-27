import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { MessagingStreamApi } from './messaging-stream-api';
import { MessagingStreamRepositoryImpl } from './messaging-stream-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * MessagingStreamRepositoryImpl spec — fixture-driven (happy + failure):
 * the stream-story payload (beats, proofs) round-trips; malformed
 * payloads and the server's honest 503 for an unreachable Redis resolve
 * as Failed Results, never throws.
 */

const demoPayload = {
  namespace: 'gdg:demo:msg:0123456789ab',
  stream: 'gdg:demo:msg:0123456789ab:orders',
  group: 'workers',
  commands: ['XADD', 'XREADGROUP', 'XACK', 'XPENDING', 'XAUTOCLAIM'],
  beats: [
    {
      beat: 'append',
      commands: ['XADD gdg:demo:msg:0123456789ab:orders … step created'],
      outcome: 'three entries appended; ids assigned by the store.',
      entries: ['1780000000000-0', '1780000000001-0', '1780000000002-0'],
    },
    {
      beat: 'consume (b)',
      commands: ['XREADGROUP GROUP workers consumer-b COUNT 10 >'],
      outcome: 'consumer-b received 0 new entries.',
      entries: [],
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

function makeRepository(): MessagingStreamRepositoryImpl {
  return new MessagingStreamRepositoryImpl(new MessagingStreamApi(createAppConfig()));
}

describe('MessagingStreamRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/msg/stream', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.namespace).toBe('gdg:demo:msg:0123456789ab');
      expect(result.value.group).toBe('workers');
      expect(result.value.beats[0]?.entries).toHaveLength(3);
      expect(result.value.proofs.pendingLeft).toBe(0);
    }
  });

  it('rejects a payload whose proofs are missing', async () => {
    const poisoned = { ...demoPayload, proofs: undefined };
    stubApiFetch([{ url: '/api/demo/msg/stream', body: poisoned }]);

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
