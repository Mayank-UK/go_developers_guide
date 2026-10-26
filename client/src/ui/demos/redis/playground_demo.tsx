import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetRedisPlaygroundDemoUseCase,
  REDIS_PLAYGROUND_DEMO_USE_CASE,
} from '../../../domain/demo/get-redis-playground-demo-usecase';
import type {
  RedisPlaygroundDemoData,
  RedisStructureRow,
} from '../../../domain/demo/redis-playground';

/**
 * RedisPlaygroundDemo — the redis topic's live demo (demo block id
 * `redis:playground`). It renders the payload of GET
 * /api/demo/redis/playground: one scripted walk against the RUNNING
 * stack Redis — the five core structures with their real commands and
 * read-backs, the pipelined INCR counter with its exact total, the TTL
 * countdown read as data (with the SET NX lock pair's two answers), one
 * REAL pub/sub round-trip reported from both sides, and the cleanup that
 * removed every key the walk created.
 *
 * Every load mints a fresh throwaway namespace server-side, so the Retry
 * button walks a genuinely new set of keys; an unreachable Redis
 * degrades to the typed error panel (the server's honest 503), never to
 * a stale answer.
 */

type PlaygroundStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: RedisPlaygroundDemoData }
  | { state: 'error'; failure: Failure };

export function useRedisPlaygroundDemo(
  useCase: GetRedisPlaygroundDemoUseCase = container.resolve(REDIS_PLAYGROUND_DEMO_USE_CASE),
): { status: PlaygroundStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<PlaygroundStatus>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });

    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus({ state: 'ready', data: result.value });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function RedisPlaygroundDemo() {
  const { status, retry } = useRedisPlaygroundDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="redis-playground-loading" className="text-body-sm text-on-surface-variant">
        walking the structures against the live store…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the redis playground" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="redis-playground-ready">
      <p className="font-mono text-caption text-on-surface-variant">
        namespace <span className="text-secondary">{data.namespace}</span> — throwaway per run,
        removed on the way out
      </p>

      {data.structures.map((structure) => (
        <StructureRun key={structure.kind} structure={structure} />
      ))}

      <section
        data-testid="redis-playground-counter"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">
          atomicity · {data.counter.key}
        </p>
        {data.counter.commands.map((command) => (
          <p key={command} className="font-mono text-caption text-on-surface-variant">
            {command}
          </p>
        ))}
        <p data-testid="redis-playground-counter-proof" className="font-mono text-caption text-on-surface">
          {data.counter.pipelineSize} pipelined INCRs → GET answered{' '}
          <span className={data.counter.total === data.counter.pipelineSize ? 'text-primary' : 'text-error'}>
            {String(data.counter.total)}
          </span>
        </p>
        <p className="text-caption text-on-surface-variant">{data.counter.proof}</p>
      </section>

      <section
        data-testid="redis-playground-ttl"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">ttl · {data.ttl.key}</p>
        {data.ttl.commands.map((command) => (
          <p key={command} className="font-mono text-caption text-on-surface-variant">
            {command}
          </p>
        ))}
        <p data-testid="redis-playground-ttl-proof" className="font-mono text-caption text-on-surface">
          configured {data.ttl.ttlSeconds}s · TTL read {data.ttl.ttlFirstRead}s · SET NX acquired{' '}
          <span className={data.ttl.nxFirstAcquired ? 'text-primary' : 'text-error'}>
            {String(data.ttl.nxFirstAcquired)}
          </span>{' '}
          · second holder refused{' '}
          <span className={!data.ttl.nxSecondAcquired ? 'text-primary' : 'text-error'}>
            {String(!data.ttl.nxSecondAcquired)}
          </span>
        </p>
        <p className="text-caption text-on-surface-variant">{data.ttl.proof}</p>
      </section>

      <section
        data-testid="redis-playground-pubsub"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">pub/sub · {data.pubsub.channel}</p>
        {data.pubsub.commands.map((command) => (
          <p key={command} className="font-mono text-caption text-on-surface-variant">
            {command}
          </p>
        ))}
        <p data-testid="redis-playground-pubsub-proof" className="font-mono text-caption text-on-surface">
          publish answered {data.pubsub.receivers} receiver(s) · payload matched:{' '}
          <span className={data.pubsub.matched ? 'text-primary' : 'text-error'}>
            {String(data.pubsub.matched)}
          </span>
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          published: {data.pubsub.payload}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          received: {data.pubsub.receivedPayload}
        </p>
        <p className="text-caption text-on-surface-variant">{data.pubsub.proof}</p>
      </section>

      <section data-testid="redis-playground-cleanup" className="flex flex-col gap-xxs px-md">
        <p className="font-mono text-caption text-secondary">
          cleanup · SCAN + DEL removed {data.cleanup.keysRemoved} keys — nothing left behind
        </p>
        <p className="text-caption text-on-surface-variant">{data.cleanup.proof}</p>
      </section>

      <p data-testid="redis-playground-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

function StructureRun({ structure }: { structure: RedisStructureRow }) {
  return (
    <section
      data-testid={`redis-playground-${structure.kind}`}
      className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
    >
      <p className="font-mono text-caption text-on-surface">
        <span className="text-secondary">{structure.kind}</span> — {structure.key}
      </p>
      {structure.commands.map((command) => (
        <p key={command} className="font-mono text-caption text-on-surface-variant">
          {command}
        </p>
      ))}
      {structure.values.map((row, index) => (
        <p key={index} className="font-mono text-caption text-on-surface-variant">
          {row.label} = {row.value}
        </p>
      ))}
      <p className="font-mono text-caption text-on-surface">{structure.outcome}</p>
      <p className="text-caption text-on-surface-variant">{structure.description}</p>
    </section>
  );
}

export default RedisPlaygroundDemo;
