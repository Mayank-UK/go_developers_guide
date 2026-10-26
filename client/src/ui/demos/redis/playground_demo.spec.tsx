import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { RedisPlaygroundDemo } from './playground_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetRedisPlaygroundDemoUseCase,
  REDIS_PLAYGROUND_DEMO_USE_CASE,
} from '../../../domain/demo/get-redis-playground-demo-usecase';
import type { RedisPlaygroundDemoData } from '../../../domain/demo/redis-playground';

/**
 * RedisPlaygroundDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the five structures render with their real commands and read-backs,
 * the counter's exact total renders, the TTL countdown renders as data
 * with the NX pair's two answers, the pub/sub round-trip renders from
 * both sides, the cleanup renders, and failures — including the server's
 * honest 503 for an unreachable Redis — degrade to the shared error
 * panel with recovery.
 */

const fixture: RedisPlaygroundDemoData = {
  namespace: 'gdg:demo:redis:0123456789ab',
  structures: [
    {
      kind: 'string',
      key: 'gdg:demo:redis:0123456789ab:greeting',
      description: 'the workhorse: one binary-safe value up to 512 MB',
      commands: ['SET gdg:demo:redis:0123456789ab:greeting "Hello, Redis — the guide\u2019s first string." EX 30', 'GET gdg:demo:redis:0123456789ab:greeting'],
      outcome: 'SET answered OK (value and TTL in one command); GET read the value straight back.',
      values: [{ label: 'GET', value: 'Hello, Redis — the guide\u2019s first string.' }],
    },
    {
      kind: 'hash',
      key: 'gdg:demo:redis:0123456789ab:post',
      description: 'one key, many field→value pairs',
      commands: ['HSET gdg:demo:redis:0123456789ab:post title … eviction …', 'HGETALL gdg:demo:redis:0123456789ab:post'],
      outcome: 'HSET answered 4 new fields; HGETALL returned all 4 — the object came back whole.',
      values: [
        { label: 'engine', value: 'in-memory' },
        { label: 'eviction', value: 'per-key TTL' },
      ],
    },
    {
      kind: 'list',
      key: 'gdg:demo:redis:0123456789ab:timeline',
      description: 'an ordered sequence addressed from both ends',
      commands: ['LPUSH gdg:demo:redis:0123456789ab:timeline first second third', 'LRANGE gdg:demo:redis:0123456789ab:timeline 0 -1'],
      outcome: 'LPUSH answered 3; LRANGE read them back newest-first — lists push at the head, so the order inverts.',
      values: [
        { label: 'LRANGE 0', value: 'third' },
        { label: 'LRANGE 1', value: 'second' },
        { label: 'LRANGE 2', value: 'first' },
      ],
    },
    {
      kind: 'set',
      key: 'gdg:demo:redis:0123456789ab:tags',
      description: 'an unordered, unique membership',
      commands: ['SADD gdg:demo:redis:0123456789ab:tags go redis server go', 'SMEMBERS gdg:demo:redis:0123456789ab:tags'],
      outcome: 'SADD answered 3 of 4 arguments — the duplicate was refused by the structure.',
      values: [
        { label: 'member', value: 'go' },
        { label: 'member', value: 'redis' },
        { label: 'member', value: 'server' },
      ],
    },
    {
      kind: 'zset',
      key: 'gdg:demo:redis:0123456789ab:leaderboard',
      description: 'members carrying scores, kept ordered by them',
      commands: ['ZADD gdg:demo:redis:0123456789ab:leaderboard 100 grace 85 ada 97 linus', 'ZRANGE gdg:demo:redis:0123456789ab:leaderboard 0 -1 WITHSCORES'],
      outcome: 'written 100, 85, 97, returned 85, 97, 100.',
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
    commands: ['DEL gdg:demo:redis:0123456789ab:views', 'PIPELINE · 25× INCR gdg:demo:redis:0123456789ab:views', 'GET gdg:demo:redis:0123456789ab:views'],
    total: 25,
    proof: '25 INCRs rode ONE pipeline (one round trip) and the final GET answered 25.',
  },
  ttl: {
    key: 'gdg:demo:redis:0123456789ab:flash',
    ttlSeconds: 30,
    commands: ['SET gdg:demo:redis:0123456789ab:flash "here for 30 seconds" EX 30', 'TTL gdg:demo:redis:0123456789ab:flash'],
    ttlFirstRead: 30,
    nxFirstAcquired: true,
    nxSecondAcquired: false,
    proof: 'SET carried its EX 30 inline and TTL read 30 back: a countdown the STORE owns.',
  },
  pubsub: {
    channel: 'gdg:demo:redis:0123456789ab:events',
    payload: 'playground event — hello from the publisher',
    commands: ['SUBSCRIBE gdg:demo:redis:0123456789ab:events', 'PUBLISH gdg:demo:redis:0123456789ab:events "playground event — hello from the publisher"'],
    receivers: 1,
    receivedPayload: 'playground event — hello from the publisher',
    matched: true,
    proof: 'both sides of one real round trip, bounded by a context deadline.',
  },
  cleanup: {
    keysRemoved: 8,
    commands: ['SCAN 0 MATCH "gdg:demo:redis:0123456789ab:*" COUNT 100', 'DEL gdg:demo:redis:0123456789ab …'],
    proof: 'the playground leaves nothing behind on the shared store.',
  },
  reasoning: 'Redis is a data STRUCTURES server — the sorting, uniqueness, counting, and ordering happened inside the store.',
};

function stubUseCase(execute: () => Promise<Result<RedisPlaygroundDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    REDIS_PLAYGROUND_DEMO_USE_CASE,
    { execute } as unknown as GetRedisPlaygroundDemoUseCase,
  );
}

describe('RedisPlaygroundDemo', () => {
  it('renders loading first, then the whole walk', async () => {
    stubUseCase(async () => success(fixture));

    render(<RedisPlaygroundDemo />);

    expect(screen.getByTestId('redis-playground-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('redis-playground-ready')).toBeInTheDocument());

    // The five structures, each with its commands, read-backs, and outcome.
    const stringRun = screen.getByTestId('redis-playground-string');
    expect(stringRun).toHaveTextContent('Hello, Redis — the guide\u2019s first string.');
    expect(stringRun).toHaveTextContent('EX 30');

    const hashRun = screen.getByTestId('redis-playground-hash');
    expect(hashRun).toHaveTextContent('the object came back whole');

    const listRun = screen.getByTestId('redis-playground-list');
    expect(listRun).toHaveTextContent('third');
    expect(listRun).toHaveTextContent('the order inverts');

    const setRun = screen.getByTestId('redis-playground-set');
    expect(setRun).toHaveTextContent('the duplicate was refused');

    const zsetRun = screen.getByTestId('redis-playground-zset');
    expect(zsetRun).toHaveTextContent('score 85');
    expect(zsetRun).toHaveTextContent('score 100');

    // The counter's exact total, the TTL countdown as data, the pub/sub
    // round-trip, the cleanup, and the namespace banner.
    expect(screen.getByTestId('redis-playground-counter')).toHaveTextContent('GET answered 25');
    expect(screen.getByTestId('redis-playground-ttl')).toHaveTextContent('configured 30s · TTL read 30s');
    expect(screen.getByTestId('redis-playground-ttl')).toHaveTextContent('SET NX acquired true');
    expect(screen.getByTestId('redis-playground-pubsub')).toHaveTextContent('1 receiver(s)');
    expect(screen.getByTestId('redis-playground-pubsub')).toHaveTextContent('received: playground event — hello from the publisher');
    expect(screen.getByTestId('redis-playground-cleanup')).toHaveTextContent('removed 8 keys');
    expect(screen.getByTestId('redis-playground-reasoning')).toHaveTextContent('STRUCTURES server');
  });

  it('degrades to the error panel on the server\u2019s honest 503, with recovery', async () => {
    const failure = new ServerFailure(
      'The redis playground runs real commands against the guide\u2019s Redis, which is not reachable right now.',
      { code: 'service_unavailable', status: 503, requestId: 'rid-redis-503' },
    );
    stubUseCase(async () => failed(failure));

    render(<RedisPlaygroundDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');
    expect(panel).toHaveTextContent('not reachable right now');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('redis-playground-ready')).toBeInTheDocument());
  });
});
