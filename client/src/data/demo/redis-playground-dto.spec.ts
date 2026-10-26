import { describe, expect, it } from 'vitest';
import { redisPlaygroundDtoFromJson, toRedisPlaygroundDemo } from './redis-playground-dto';
import type { RedisPlaygroundDemoData } from '../../domain/demo/redis-playground';

/**
 * RedisPlaygroundDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). The TTL
 * countdown varies with the store's live clock by design — asserted by
 * shape only, never pinned to a literal.
 */

const wellFormed: RedisPlaygroundDemoData = {
  namespace: 'gdg:demo:redis:0123456789ab',
  structures: [
    {
      kind: 'string',
      key: 'gdg:demo:redis:0123456789ab:greeting',
      description: 'the workhorse: one binary-safe value up to 512 MB',
      commands: ['SET gdg:demo:redis:0123456789ab:greeting "Hello" EX 30', 'GET gdg:demo:redis:0123456789ab:greeting'],
      outcome: 'SET answered OK; GET read the value straight back.',
      values: [{ label: 'GET', value: 'Hello, Redis — the guide\u2019s first string.' }],
    },
    {
      kind: 'zset',
      key: 'gdg:demo:redis:0123456789ab:leaderboard',
      description: 'members carrying scores, kept ordered by them',
      commands: ['ZADD …', 'ZRANGE … WITHSCORES'],
      outcome: 'written 100, 85, 97; returned 85, 97, 100.',
      values: [
        { label: 'ada', value: 'score 85' },
        { label: 'linus', value: 'score 97' },
        { label: 'grace', value: 'score 100' },
      ],
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
    commands: ['SET … EX 30', 'TTL …', 'SET … NX', 'SET … NX'],
    ttlFirstRead: 30,
    nxFirstAcquired: true,
    nxSecondAcquired: false,
    proof: 'the first SET NX acquired (OK), the second was refused (nil).',
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
  reasoning: 'Every row above is what the live store answered.',
};

describe('redisPlaygroundDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(redisPlaygroundDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps the identity conversion stable', () => {
    expect(toRedisPlaygroundDemo(wellFormed)).toBe(wellFormed);
  });

  it('accepts the live countdown wherever the store clock is (shape, not literal)', () => {
    const drifted = { ...wellFormed, ttl: { ...wellFormed.ttl, ttlFirstRead: 29 } };
    expect(redisPlaygroundDtoFromJson(drifted).ttl.ttlFirstRead).toBe(29);
  });

  it('rejects an unknown structure kind', () => {
    const poisoned = {
      ...wellFormed,
      structures: [{ ...wellFormed.structures[0], kind: 'bloom' }],
    };
    expect(() => redisPlaygroundDtoFromJson(poisoned)).toThrow(/unknown structure kind/);
  });

  it('rejects a missing proof object', () => {
    const poisoned = { ...wellFormed } as Record<string, unknown>;
    delete poisoned.counter;
    expect(() => redisPlaygroundDtoFromJson(poisoned)).toThrow(/counter/);
  });

  it('rejects a non-boolean NX answer', () => {
    const poisoned = {
      ...wellFormed,
      ttl: { ...wellFormed.ttl, nxSecondAcquired: 'no' },
    };
    expect(() => redisPlaygroundDtoFromJson(poisoned)).toThrow(/nxSecondAcquired/);
  });

  it('rejects a non-array commands list', () => {
    const poisoned = {
      ...wellFormed,
      pubsub: { ...wellFormed.pubsub, commands: 'SUBSCRIBE …' },
    };
    expect(() => redisPlaygroundDtoFromJson(poisoned)).toThrow(/commands/);
  });

  it('rejects a non-object body outright', () => {
    expect(() => redisPlaygroundDtoFromJson('nope')).toThrow(/must be an object/);
    expect(() => redisPlaygroundDtoFromJson(null)).toThrow(/must be an object/);
  });
});
