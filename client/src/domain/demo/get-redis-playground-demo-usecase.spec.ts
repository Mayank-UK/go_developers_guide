import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { RedisPlaygroundDemoData } from './redis-playground';
import type { RedisPlaygroundRepository } from './redis-playground-repository';
import { GetRedisPlaygroundDemoUseCase } from './get-redis-playground-demo-usecase';

/**
 * GetRedisPlaygroundDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

function fakeRepository(get: Result<RedisPlaygroundDemoData>): RedisPlaygroundRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: RedisPlaygroundDemoData = {
  namespace: 'gdg:demo:redis:0123456789ab',
  structures: [
    {
      kind: 'string',
      key: 'gdg:demo:redis:0123456789ab:greeting',
      description: 'the workhorse',
      commands: ['SET … EX 30', 'GET …'],
      outcome: 'SET answered OK; GET read the value straight back.',
      values: [{ label: 'GET', value: 'Hello, Redis — the guide\u2019s first string.' }],
    },
  ],
  counter: {
    key: 'gdg:demo:redis:0123456789ab:views',
    pipelineSize: 25,
    commands: ['DEL …', 'PIPELINE · 25× INCR …', 'GET …'],
    total: 25,
    proof: '25 INCRs rode ONE pipeline and the final GET answered 25.',
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

describe('GetRedisPlaygroundDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetRedisPlaygroundDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetRedisPlaygroundDemoUseCase(
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
