import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetCachingLabDemoUseCase,
  CACHING_LAB_DEMO_USE_CASE,
} from '../../../domain/demo/get-caching-lab-demo-usecase';
import type {
  CachingLabBeat,
  CachingLabDemoData,
} from '../../../domain/demo/caching-lab';

/**
 * CachingLabDemo — the caching topic's live demo (demo block id
 * `caching:lab`). It renders the payload of GET /api/demo/caching/lab: the
 * whole cache-aside lifecycle run through the REAL decorator on throwaway
 * stores — the beats (cold read, warm hit, cached page, write
 * invalidation, TTL expiry on a fixture clock), the twelve-reader herd
 * coalesced into one store read, the fake/live store swap that stayed
 * partition-honest, and the final scoreboard. The payload is deterministic
 * by construction, which is why the server can certify it with an ETag and
 * answer a replayed If-None-Match with a 304 — the round trip the topic's
 * terminal companion curls by hand.
 */

type LabStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: CachingLabDemoData }
  | { state: 'error'; failure: Failure };

export function useCachingLabDemo(
  useCase: GetCachingLabDemoUseCase = container.resolve(CACHING_LAB_DEMO_USE_CASE),
): { status: LabStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<LabStatus>({ state: 'idle' });

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

export function CachingLabDemo() {
  const { status, retry } = useCachingLabDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="caching-lab-loading" className="text-body-sm text-on-surface-variant">
        running the cache-aside beats against the real decorator…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the caching lab" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="caching-lab-ready">
      <p className="font-mono text-caption text-on-surface-variant">
        ttl {data.ttlSeconds}s · fixture clock · payload deterministic — the response carries its
        own ETag
      </p>

      {data.beats.map((beat) => (
        <BeatRun key={beat.id} beat={beat} />
      ))}

      <section
        data-testid="caching-lab-herd"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">
          the herd · {data.herd.readers} concurrent readers of {data.herd.id}
        </p>
        <p data-testid="caching-lab-herd-proof" className="font-mono text-caption text-on-surface">
          {data.herd.readers} readers →{' '}
          <span className={data.herd.storeReads === 1 ? 'text-primary' : 'text-error'}>
            {String(data.herd.storeReads)}
          </span>{' '}
          store read(s) · same answer{' '}
          <span className={data.herd.sameAnswer ? 'text-primary' : 'text-error'}>
            {String(data.herd.sameAnswer)}
          </span>{' '}
          · {data.herd.answer}
        </p>
        <p className="text-caption text-on-surface-variant">{data.herd.proof}</p>
      </section>

      <section
        data-testid="caching-lab-swap"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">
          the swap · fake ↔ live through one cache
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          live-only {data.swap.liveOnlyId}: {data.swap.liveOnlyTitle} · in fake:{' '}
          <span className={data.swap.notFoundSurfaced ? 'text-primary' : 'text-error'}>
            typed miss {String(data.swap.notFoundSurfaced)}
          </span>
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          fake p-1: {data.swap.fakeTitle}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          live p-1: {data.swap.liveTitle}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          back in fake: {data.swap.backTitle}
        </p>
        <p className="text-caption text-on-surface-variant">{data.swap.proof}</p>
      </section>

      <section
        data-testid="caching-lab-stats"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the scoreboard</p>
        <p data-testid="caching-lab-stats-line" className="font-mono text-caption text-on-surface">
          {data.stats.hits} hits · {data.stats.misses} misses · {data.stats.expiries} expiries ·{' '}
          {data.stats.invalidations} invalidations · {data.stats.storeReads} store reads ·{' '}
          {data.stats.entries} entries
        </p>
        <p className="text-caption text-on-surface-variant">{data.stats.proof}</p>
      </section>

      <p data-testid="caching-lab-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

function BeatRun({ beat }: { beat: CachingLabBeat }) {
  return (
    <section
      data-testid={`caching-lab-beat-${beat.id}`}
      className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
    >
      <p className="font-mono text-caption text-on-surface">
        <span className="text-secondary">{beat.outcome}</span> — {beat.title}
      </p>
      {beat.script.map((line) => (
        <p key={line} className="font-mono text-caption text-on-surface-variant">
          {line}
        </p>
      ))}
      {beat.observedValue !== '' && (
        <p className="font-mono text-caption text-on-surface-variant">
          read: {beat.observedValue}
        </p>
      )}
      <p className="text-caption text-on-surface-variant">{beat.detail}</p>
      <p className="font-mono text-caption text-on-surface-variant">
        hits {beat.hits} · misses {beat.misses} · store reads {beat.storeReads}
      </p>
    </section>
  );
}

export default CachingLabDemo;
