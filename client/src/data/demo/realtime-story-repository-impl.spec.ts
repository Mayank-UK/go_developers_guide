import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { RealtimeStoryApi } from './realtime-story-api';
import { RealtimeStoryRepositoryImpl } from './realtime-story-repository-impl';
import { REALTIME_STORY_PATHS } from './realtime-story-dto';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * RealtimeStoryRepositoryImpl spec — fixture-driven (happy + failure)
 * over BOTH endpoint paths the container parameterizes: the story
 * payload (beats with verdicts and derived facts, proofs, reasoning)
 * round-trips; a poisoned payload and a network failure resolve as
 * Failed Results, never throws.
 */

const tickerPayload = {
  mechanism: 'sse-ticker',
  beats: [
    {
      beat: 'the frame anatomy',
      detail: 'field lines ended by one blank line; the live stream emits exactly these bytes for every tick.',
      verdict: 'ok',
      facts: [{ label: 'a real tick frame', value: 'id: 41\nevent: tick\ndata: {"seq":41}\n\n' }],
    },
    {
      beat: 'Last-Event-ID resume',
      detail: 'a client that last processed 38 gets exactly three frames back, then live.',
      verdict: 'ok',
      facts: [{ label: 'Last-Event-ID: 38', value: '3 frames replayed — seqs 39, 40, 41 — then live' }],
    },
  ],
  proofs: [{ label: 'one-way only', value: 'SSE is server→client over plain HTTP' }],
  reasoning: 'the server flushes, numbers, retains, and keeps the line warm.',
};

const chatPayload = {
  mechanism: 'ws-chat',
  beats: [
    {
      beat: 'the slow consumer',
      detail: 'the stalled subscriber missed two broadcasts with a full buffer and was evicted.',
      verdict: 'rejected',
      facts: [{ label: 'publish #4', value: 'seq 4: the drop budget hit — evicted: [stalled]' }],
    },
  ],
  proofs: [{ label: 'eviction math', value: 'live policy: buffer 16 + budget 8 drops → close 1013' }],
  reasoning: 'bounded fan-out, explicit slow-consumer policy, honest teardown.',
};

describe('RealtimeStoryRepositoryImpl', () => {
  it('round-trips the ticker story from its own endpoint path', async () => {
    stubApiFetch([{ url: '/api/demo/realtime/ticker', body: tickerPayload }]);

    const repository = new RealtimeStoryRepositoryImpl(
      new RealtimeStoryApi(createAppConfig(), REALTIME_STORY_PATHS.ticker),
    );
    const result = await repository.get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.mechanism).toBe('sse-ticker');
      expect(result.value.beats[1]?.facts[0]?.value).toContain('39, 40, 41');
      expect(result.value.proofs[0]?.label).toBe('one-way only');
    }
  });

  it('round-trips the chat story from its own endpoint path', async () => {
    stubApiFetch([{ url: '/api/demo/realtime/chat', body: chatPayload }]);

    const repository = new RealtimeStoryRepositoryImpl(
      new RealtimeStoryApi(createAppConfig(), REALTIME_STORY_PATHS.chat),
    );
    const result = await repository.get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.mechanism).toBe('ws-chat');
      expect(result.value.beats[0]?.verdict).toBe('rejected');
      expect(result.value.reasoning).toContain('honest teardown');
    }
  });

  it('rejects a payload whose beat verdict is unknown', async () => {
    const poisoned = {
      ...chatPayload,
      beats: [{ ...chatPayload.beats[0], verdict: 'maybe' }],
    };
    stubApiFetch([{ url: '/api/demo/realtime/chat', body: poisoned }]);

    const repository = new RealtimeStoryRepositoryImpl(
      new RealtimeStoryApi(createAppConfig(), REALTIME_STORY_PATHS.chat),
    );
    const result = await repository.get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const repository = new RealtimeStoryRepositoryImpl(
      new RealtimeStoryApi(createAppConfig(), REALTIME_STORY_PATHS.ticker),
    );
    const result = await repository.get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
