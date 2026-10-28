import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ArchEventstoreDemo } from './eventstore_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetArchEventstoreDemoUseCase,
  ARCH_EVENTSTORE_DEMO_USE_CASE,
} from '../../../domain/demo/get-arch-eventstore-demo-usecase';
import type { ArchEventstoreDemoData } from '../../../domain/demo/arch-eventstore';

/**
 * ArchEventstoreDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the store surface and every panel render with their observed rows,
 * versions, outcomes, and counted totals; the conflict row renders its
 * expected/actual pair; the replay evidence renders the fold order, the
 * rebuilt state beside the naive ledger, and the absent stream's
 * zero-state fold; and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: ArchEventstoreDemoData = {
  mode: 'event store lifecycle: one inventory stream driven through the real EventStore',
  honestyNote: 'No-invention pin: every version was observed from the real store.',
  streamId: 'inventory-tee-medium',
  store: [
    'EventStore.Append(ctx, streamID, expectedVersion, events...) — the ONLY write path',
    'EventProject(ctx, store, streamID, initial, apply) → EventProjection{State, Version}',
  ],
  panels: [
    {
      id: 'lifecycle',
      title: 'Act one — one writer, the command path',
      lead: 'Every command rebuilds by replay, decides, appends naming its expected version.',
      config: ['the loop: Project (replay) → decide → Append(expected = projection.Version)'],
      rows: [
        {
          step: 1,
          call: 'Append (stock)',
          input: 'expected=EventStreamVersionEmpty (0), 1 event: inventory.stocked qty=50',
          outcome: 'appended',
          version: 1,
          detail: 'inventory.stocked landed as v1; replay now says onHand 50 · reserved 0 · available 50 · open {}',
        },
        {
          step: 2,
          call: 'Append (reserve)',
          input: 'expected=1, inventory.reserved qty=20 ref=order-A',
          outcome: 'appended',
          version: 2,
          detail: 'inventory.reserved landed as v2; replay now says onHand 50 · reserved 20 · available 30 · open {order-A:20}',
        },
      ],
      counts: [
        { label: 'appends accepted', value: 2 },
        { label: 'events stored', value: 2 },
      ],
    },
    {
      id: 'safety',
      title: 'Act two — two writers, the conflict, the recovery',
      lead: 'Optimistic concurrency is the guardrail between a read and its follow-up write.',
      config: ['a refused append lands nothing'],
      rows: [
        {
          step: 7,
          call: 'Append (writer two, stale)',
          input: 'expected=3 (its snapshot), inventory.released qty=20 ref=order-A',
          outcome: 'refused-conflict',
          version: 4,
          expectedVersion: 3,
          actualVersion: 4,
          detail: '*EventStoreConflict observed via errors.As: expected 3, actual 4 — nothing landed',
        },
        {
          step: 9,
          call: 'Append (writer two, recovered)',
          input: 'expected=4 (the reloaded head), inventory.released qty=25 ref=order-B',
          outcome: 'appended-after-reload',
          version: 5,
          detail: 'inventory.released landed as v5; replay now says onHand 30 · reserved 0 · available 30 · open {}',
        },
      ],
      counts: [{ label: 'optimistic conflicts (typed) observed', value: 1 }],
    },
    {
      id: 'replay',
      title: 'Act three — the rebuild proofs',
      lead: 'The state was never stored; these rows rebuild it from the events alone.',
      config: ['replay = EventFold over the records in version order'],
      rows: [
        {
          step: 13,
          call: 'Project (absent stream)',
          input: 'stream "inventory-never-stocked" — never appended to',
          outcome: 'zero-state-fold',
          version: 0,
          detail: "no 'missing' error: an absent stream folds to the zero state",
        },
      ],
      counts: [{ label: 'rebuilds run', value: 2 }],
    },
  ],
  replay: {
    foldedEvents: 5,
    appliedInOrder: [
      'v1 inventory.stocked {"qty":50}@2026-10-28T12:00:00Z',
      'v2 inventory.reserved {"qty":20,"ref":"order-A"}@2026-10-28T12:00:30Z',
    ],
    state: { onHand: 30, reserved: 0, available: 30, reservations: {} },
    naiveExpected: { onHand: 30, reserved: 0, available: 30, reservations: {} },
    rebuildsRun: 2,
    byteIdentical: true,
    absentStreamId: 'inventory-never-stocked',
    absentState: { onHand: 0, reserved: 0, available: 0, reservations: {} },
    absentVersion: 0,
    note: 'the fold consumed the events above, in exactly that order',
  },
};

function stubUseCase(execute: () => Promise<Result<ArchEventstoreDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    ARCH_EVENTSTORE_DEMO_USE_CASE,
    { execute } as unknown as GetArchEventstoreDemoUseCase,
  );
}

describe('ArchEventstoreDemo', () => {
  it('renders loading first, then the store surface, every act, and the evidence', async () => {
    stubUseCase(async () => success(fixture));

    render(<ArchEventstoreDemo />);

    expect(screen.getByTestId('arch-eventstore-loading')).toBeInTheDocument();

    await waitForReady();
    const surface = screen.getByTestId('arch-eventstore-store');
    expect(surface).toHaveTextContent('inventory-tee-medium');
    expect(surface).toHaveTextContent('the ONLY write path');

    for (const id of ['lifecycle', 'safety', 'replay'] as const) {
      expect(screen.getByTestId(`es-panel-${id}`)).toBeInTheDocument();
    }

    const firstAppend = screen.getByTestId('es-row-1');
    expect(firstAppend).toHaveTextContent('appended');
    expect(firstAppend).toHaveTextContent('v1');
    expect(firstAppend).toHaveTextContent('onHand 50');

    const conflict = screen.getByTestId('es-row-7');
    expect(conflict).toHaveTextContent('refused-conflict');
    expect(conflict).toHaveTextContent('expected version 3, actual 4 — nothing landed');

    const absent = screen.getByTestId('es-row-13');
    expect(absent).toHaveTextContent('zero-state-fold');
    expect(absent).toHaveTextContent('v0');

    expect(screen.getAllByTestId('es-count-safety').length).toBeGreaterThan(0);

    const evidence = screen.getByTestId('arch-eventstore-evidence');
    expect(evidence).toHaveTextContent('5 events folded, in order');
    expect(evidence).toHaveTextContent('v1 inventory.stocked');
    expect(evidence).toHaveTextContent('rebuilt: onHand 30 · reserved 0 · available 30 · open {}');
    expect(evidence).toHaveTextContent('naive ledger (no fold, no store)');
    expect(evidence).toHaveTextContent('2 rebuilds, byte-identical=true');
    expect(evidence).toHaveTextContent('absent stream inventory-never-stocked');

    expect(screen.getByTestId('arch-eventstore-honesty')).toHaveTextContent('No-invention pin');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure('The event-store exhibit could not be reached.', {
      code: 'network_error',
      status: 0,
      requestId: 'rid-es-down',
    });
    stubUseCase(async () => failed(failure));

    render(<ArchEventstoreDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('arch-eventstore-ready')).toBeInTheDocument());
}
