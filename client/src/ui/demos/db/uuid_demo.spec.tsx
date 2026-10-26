import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { UUIDLabDemo } from './uuid_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { NetworkFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  DB_UUID_DEMO_USE_CASE,
  GetUUIDLabDemoUseCase,
} from '../../../domain/demo/get-uuid-lab-demo-usecase';
import type { UUIDLabDemoData } from '../../../domain/demo/uuid-lab';

/**
 * UUIDLabDemo spec — fixture-driven (happy + failure). The uuid VALUES
 * in fixtures are property-valid shapes, never pinned literals (every
 * load mints fresh randomness — asserting literals would be asserting
 * randomness). What is pinned: the layout table renders, the anatomy
 * card shows version/variant/length, the batch renders generated beside
 * sorted with the sortedness verdict, the collision check renders its
 * distinct-of-generated count, the comparison table renders all three
 * strategies — and failures degrade to the shared error panel.
 */

const A = '0192ac15-0211-7001-90d1-d2d3d4d5d6d7';
const B = '0192ac15-0211-7002-90d2-d2d3d4d5d6d7';
const C = '0192ac15-0211-7003-90d3-d2d3d4d5d6d7';
const D = '0192ac15-0211-7004-90d4-d2d3d4d5d6d7';

const fixture: UUIDLabDemoData = {
  generator: {
    name: 'UUIDv7, hand-rolled (RFC 9562 §5.7)',
    randomSource: 'crypto/rand — 12-bit counter seed + 62 random bits per id',
    description: '48-bit Unix milliseconds + version 7 + monotonic 12-bit counter + variant 10 + 62 random bits',
  },
  layout: [
    { field: 'unix_ts_ms', bits: 48, contents: 'Unix epoch milliseconds, big-endian — the sort key' },
    { field: 'ver', bits: 4, contents: '0b0111 — version 7' },
    { field: 'rand_a', bits: 12, contents: 'a per-millisecond counter' },
    { field: 'var', bits: 2, contents: '0b10 — the RFC 4122/9562 variant marker' },
    { field: 'rand_b', bits: 62, contents: 'crypto/rand — uniqueness' },
  ],
  anatomy: { value: A, unixMs: 1700000000001, version: 7, variant: 'RFC 4122/9562 (0b10)', length: 36 },
  batch: { count: 4, generatedInOrder: [A, B, C, D], sortedLexically: [A, B, C, D], sortingMatchesGenerationOrder: true },
  collision: { generated: 10000, distinct: 10000, collisionFree: true, randomnessBits: 74 },
  comparison: [
    {
      strategy: 'bigserial',
      storage: '8 bytes',
      indexLocality: 'perfect',
      urlGuessability: 'enumerable',
      uniquenessScope: "one table's sequence",
      notes: 'cheap and ordered, but ids leak volume and pace',
    },
    {
      strategy: 'uuidv7 (this generator)',
      storage: '16 bytes',
      indexLocality: 'good',
      urlGuessability: 'unguessable',
      uniquenessScope: 'global',
      notes: 'the default here: minted app-side, sortable, safe to expose',
    },
    {
      strategy: 'uuidv4 (gen_random_uuid())',
      storage: '16 bytes',
      indexLocality: 'worst',
      urlGuessability: 'unguessable',
      uniquenessScope: 'global',
      notes: 'PostgreSQL\u2019s built-in DEFAULT; hostile to large hot indexes',
    },
  ],
};

function stubUseCase(execute: () => Promise<Result<UUIDLabDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(DB_UUID_DEMO_USE_CASE, { execute } as unknown as GetUUIDLabDemoUseCase);
}

describe('UUIDLabDemo', () => {
  it('renders loading first, then the full lab', async () => {
    stubUseCase(async () => success(fixture));

    render(<UUIDLabDemo />);

    expect(screen.getByTestId('uuid-lab-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('uuid-lab-ready')).toBeInTheDocument());

    // The dissected fresh value: canonical form + the anatomy facts.
    const anatomy = screen.getByTestId('ul-anatomy');
    expect(anatomy).toHaveTextContent(A);
    expect(anatomy).toHaveTextContent('ver 7');
    expect(anatomy).toHaveTextContent('RFC 4122/9562 (0b10)');
    expect(anatomy).toHaveTextContent('36 chars');

    // The layout, all five fields.
    const layout = screen.getByTestId('ul-layout');
    expect(layout).toHaveTextContent('unix_ts_ms');
    expect(layout).toHaveTextContent('rand_b');

    // The batch with its verdict.
    const batch = screen.getByTestId('ul-batch');
    expect(batch).toHaveTextContent('4 ids');
    expect(batch).toHaveTextContent(B);
    expect(screen.getByTestId('ul-sorted-verdict')).toHaveTextContent('Sorted lexically == minted order');

    // The collision check.
    expect(screen.getByTestId('ul-collision')).toHaveTextContent('10000 distinct of 10000 minted');

    // The comparison, all three strategies.
    const comparison = screen.getByTestId('ul-comparison');
    expect(comparison).toHaveTextContent('bigserial');
    expect(comparison).toHaveTextContent('uuidv7 (this generator)');
    expect(comparison).toHaveTextContent('uuidv4 (gen_random_uuid())');
  });

  it('shows the mismatch verdict when sortedness breaks (the demo is honest, not decorative)', async () => {
    stubUseCase(async () =>
      success({
        ...fixture,
        batch: {
          ...fixture.batch,
          generatedInOrder: [A, C, B, D],
          sortedLexically: [A, B, C, D],
          sortingMatchesGenerationOrder: false,
        },
      }),
    );

    render(<UUIDLabDemo />);

    await waitFor(() => expect(screen.getByTestId('ul-sorted-verdict')).toBeInTheDocument());
    expect(screen.getByTestId('ul-sorted-verdict')).toHaveTextContent('Mismatch');
  });

  it('degrades to the error panel on transport failure', async () => {
    stubUseCase(async () => failed(new NetworkFailure('Could not reach the server.')));

    render(<UUIDLabDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');
  });
});
