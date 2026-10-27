import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { CachingLabDemo } from './lab_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetCachingLabDemoUseCase,
  CACHING_LAB_DEMO_USE_CASE,
} from '../../../domain/demo/get-caching-lab-demo-usecase';
import type { CachingLabDemoData } from '../../../domain/demo/caching-lab';

/**
 * CachingLabDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration instead of
 * stubbing fetch. Behavior, not implementation: every beat renders with its
 * script, outcome, read-back, and counters; the herd census renders its
 * one-read proof; the swap renders all three title observations plus the
 * typed miss; the scoreboard renders with the misses-equal-store-reads
 * invariant; failures degrade to the shared error panel with recovery.
 */

const fixture: CachingLabDemoData = {
  ttlSeconds: 5,
  beats: [
    {
      id: 'cold-read',
      title: 'The first read misses',
      script: ['GetByID("p-1")'],
      outcome: 'miss',
      observedValue: 'Why Go for backends',
      detail: 'The cache was cold: the store answered "Why Go for backends" and the entry went in (1 store reads so far).',
      hits: 0,
      misses: 1,
      storeReads: 1,
    },
    {
      id: 'warm-read',
      title: 'The second read hits',
      script: ['GetByID("p-1")'],
      outcome: 'hit',
      observedValue: 'Why Go for backends',
      detail: 'Served "Why Go for backends" from memory — the store-read counter did not move (1 total).',
      hits: 1,
      misses: 1,
      storeReads: 1,
    },
    {
      id: 'ttl-expiry',
      title: 'The fixture clock steps past the TTL',
      script: ['clock.Advance(6s)', 'GetByID("p-1")'],
      outcome: 'expired',
      observedValue: 'Why Go for backends (edited by the lab)',
      detail: 'The fixture clock stepped past the 5-second TTL: the entry died, the store re-answered.',
      hits: 1,
      misses: 2,
      storeReads: 5,
    },
  ],
  herd: {
    id: 'p-2',
    readers: 12,
    storeReads: 1,
    sameAnswer: true,
    answer: 'ServeMux patterns in Go 1.22',
    proof: '12 concurrent readers of p-2 produced exactly 1 store read — the herd coalesced.',
  },
  swap: {
    liveOnlyId: 'p-7',
    liveOnlyTitle: 'A seventh post, live only',
    notFoundId: 'p-7',
    notFoundSurfaced: true,
    fakeTitle: 'Why Go for backends (edited by the lab)',
    liveTitle: 'Why Go for backends',
    backTitle: 'Why Go for backends (edited by the lab)',
    proof: 'one store\u2019s rows never answered for the other — the data mode is salted into every key.',
  },
  stats: {
    hits: 13,
    misses: 10,
    expiries: 2,
    invalidations: 2,
    storeReads: 10,
    entries: 5,
    proof: '10 misses and 10 store reads — equal by construction.',
  },
  reasoning:
    'Every row above ran through the guide\u2019s real cache-aside decorator — and this very response carries an ETag.',
};

function stubUseCase(execute: () => Promise<Result<CachingLabDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    CACHING_LAB_DEMO_USE_CASE,
    { execute } as unknown as GetCachingLabDemoUseCase,
  );
}

describe('CachingLabDemo', () => {
  it('renders loading first, then the whole lab', async () => {
    stubUseCase(async () => success(fixture));

    render(<CachingLabDemo />);

    expect(screen.getByTestId('caching-lab-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('caching-lab-ready')).toBeInTheDocument());

    // Every beat with its outcome, script, read-back, and counters.
    const cold = screen.getByTestId('caching-lab-beat-cold-read');
    expect(cold).toHaveTextContent('miss');
    expect(cold).toHaveTextContent('GetByID("p-1")');
    expect(cold).toHaveTextContent('Why Go for backends');
    const warm = screen.getByTestId('caching-lab-beat-warm-read');
    expect(warm).toHaveTextContent('hit');
    expect(warm).toHaveTextContent('did not move');
    const expiry = screen.getByTestId('caching-lab-beat-ttl-expiry');
    expect(expiry).toHaveTextContent('clock.Advance(6s)');
    expect(expiry).toHaveTextContent('the entry died');

    // The herd census, the swap's three titles plus the typed miss, the
    // scoreboard's invariant, and the reasoning's ETag close.
    expect(screen.getByTestId('caching-lab-herd')).toHaveTextContent('12 concurrent readers');
    expect(screen.getByTestId('caching-lab-herd-proof')).toHaveTextContent('12 readers → 1 store read');
    expect(screen.getByTestId('caching-lab-swap')).toHaveTextContent('A seventh post, live only');
    expect(screen.getByTestId('caching-lab-swap')).toHaveTextContent('typed miss true');
    expect(screen.getByTestId('caching-lab-swap')).toHaveTextContent('Why Go for backends (edited by the lab)');
    expect(screen.getByTestId('caching-lab-stats')).toHaveTextContent('13 hits');
    expect(screen.getByTestId('caching-lab-stats-line')).toHaveTextContent('10 misses');
    expect(screen.getByTestId('caching-lab-stats')).toHaveTextContent('equal by construction');
    expect(screen.getByTestId('caching-lab-reasoning')).toHaveTextContent('ETag');
  });

  it('degrades to the error panel on the server\u2019s honest failure, with recovery', async () => {
    const failure = new ServerFailure(
      'The caching lab could not run its choreography.',
      { code: 'internal', status: 500, requestId: 'rid-lab-500' },
    );
    stubUseCase(async () => failed(failure));

    render(<CachingLabDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('internal');
    expect(panel).toHaveTextContent('could not run its choreography');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('caching-lab-ready')).toBeInTheDocument());
  });
});
