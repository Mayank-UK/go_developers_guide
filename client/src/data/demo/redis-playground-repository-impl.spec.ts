import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { RedisPlaygroundApi } from './redis-playground-api';
import { RedisPlaygroundRepositoryImpl } from './redis-playground-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * RedisPlaygroundRepositoryImpl spec — fixture-driven (happy + failure):
 * the playground payload (structures, counter, TTL, pub/sub, cleanup)
 * round-trips; malformed payloads and the server's honest 503 for an
 * unreachable Redis resolve as Failed Results, never throws.
 */

const demoPayload = {
  namespace: 'gdg:demo:redis:0123456789ab',
  structures: [
    {
      kind: 'string',
      key: 'gdg:demo:redis:0123456789ab:greeting',
      description: 'the workhorse',
      commands: ['SET gdg:demo:redis:0123456789ab:greeting "Hello" EX 30', 'GET gdg:demo:redis:0123456789ab:greeting'],
      outcome: 'SET answered OK; GET read the value straight back.',
      values: [{ label: 'GET', value: 'Hello, Redis — the guide\u2019s first string.' }],
    },
  ],
  counter: {
    key: 'gdg:demo:redis:0123456789ab:views',
    pipelineSize: 25,
    commands: ['DEL …', 'PIPELINE · 25× INCR …', 'GET …'],
    total: 25,
    proof: 'the final GET answered 25 — exact, no lock in application code.',
  },
  ttl: {
    key: 'gdg:demo:redis:0123456789ab:flash',
    ttlSeconds: 30,
    commands: ['SET … EX 30', 'TTL …'],
    ttlFirstRead: 30,
    nxFirstAcquired: true,
    nxSecondAcquired: false,
    proof: 'the countdown the STORE owns, served as data.',
  },
  pubsub: {
    channel: 'gdg:demo:redis:0123456789ab:events',
    payload: 'playground event — hello from the publisher',
    commands: ['SUBSCRIBE …', 'PUBLISH …'],
    receivers: 1,
    receivedPayload: 'playground event — hello from the publisher',
    matched: true,
    proof: 'both sides of one real round trip.',
  },
  cleanup: {
    keysRemoved: 8,
    commands: ['SCAN 0 MATCH …', 'DEL …'],
    proof: 'the playground leaves nothing behind on the shared store.',
  },
  reasoning: 'Redis is a data STRUCTURES server.',
};

function makeRepository(): RedisPlaygroundRepositoryImpl {
  return new RedisPlaygroundRepositoryImpl(new RedisPlaygroundApi(createAppConfig()));
}

describe('RedisPlaygroundRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/redis/playground', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.namespace).toBe('gdg:demo:redis:0123456789ab');
      expect(result.value.structures[0]?.kind).toBe('string');
      expect(result.value.structures[0]?.values[0]?.value).toContain('Hello, Redis');
      expect(result.value.counter.total).toBe(result.value.counter.pipelineSize);
      expect(result.value.ttl.nxFirstAcquired).toBe(true);
      expect(result.value.ttl.nxSecondAcquired).toBe(false);
      expect(result.value.pubsub.receivedPayload).toBe(result.value.pubsub.payload);
      expect(result.value.cleanup.keysRemoved).toBe(8);
    }
  });

  it('rejects a payload whose structure kind is unknown', async () => {
    const poisoned = {
      ...demoPayload,
      structures: [{ ...demoPayload.structures[0], kind: 'bloom' }],
    };
    stubApiFetch([{ url: '/api/demo/redis/playground', body: poisoned }]);

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
