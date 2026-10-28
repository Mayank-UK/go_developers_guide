import { describe, expect, it } from 'vitest';
import { archEventstoreDtoFromJson } from './arch-eventstore-dto';

/**
 * ArchEventstoreDTO spec — the fromJson whitelist: a full exhibit
 * payload builds the domain shape (the three panels with their observed
 * rows and counts, the replay evidence block, the store surface), a
 * NON-conflict row's absent optional fields decode as undefined
 * (omitempty on the wire — empty is DATA for the strings that can be
 * empty), and every violation (missing panels, wrong types,
 * non-numeric reservations, non-boolean byteIdentical) throws so
 * parseDto can upgrade it to an InvalidResponseException.
 */

const fullPayload = {
  mode: 'event store lifecycle: one inventory stream driven through the real EventStore',
  honestyNote: 'No-invention pin: every version, count, state, and conflict number was observed.',
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
      counts: [{ label: 'appends accepted', value: 2 }],
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
          step: 10,
          call: 'Project (rebuild)',
          input: '5 events folded from the zero state, in version order',
          outcome: 'rebuilt',
          version: 5,
          detail: 'onHand 30 · reserved 0 · available 30 · open {} — state is a DERIVATION',
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

/**
 * Checked narrows for the hostile mutators below — they reach deep into
 * the unknown-shaped fixture, so every hop is verified at runtime and a
 * broken fixture fails loudly instead of type-erasing its way through.
 */
function arrayOf(value: unknown, path: string): unknown[] {
  if (!Array.isArray(value)) throw new Error(`fixture: ${path} is not an array`);
  return value;
}

function recordOf(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null) {
    throw new Error(`fixture: ${path} is not a record`);
  }
  return value as Record<string, unknown>;
}

describe('arch-eventstore-dto', () => {
  it('builds the domain shape from a full exhibit payload (whitelist)', () => {
    const demo = archEventstoreDtoFromJson(structuredClone(fullPayload));

    expect(demo.streamId).toBe('inventory-tee-medium');
    expect(demo.store[0]).toContain('the ONLY write path');

    const lifecycle = demo.panels[0];
    expect(lifecycle?.id).toBe('lifecycle');
    const appended = lifecycle?.rows[1];
    expect(appended?.version).toBe(2);
    // A NON-conflict row carries no conflict fields — omitempty decodes as undefined.
    expect(appended?.expectedVersion).toBeUndefined();
    expect(appended?.actualVersion).toBeUndefined();
    expect(lifecycle?.counts[0]).toEqual({ label: 'appends accepted', value: 2 });

    const safety = demo.panels[1];
    const conflict = safety?.rows[0];
    expect(conflict?.outcome).toBe('refused-conflict');
    expect(conflict?.expectedVersion).toBe(3);
    expect(conflict?.actualVersion).toBe(4);

    expect(demo.replay.foldedEvents).toBe(5);
    expect(demo.replay.byteIdentical).toBe(true);
    expect(demo.replay.state.onHand).toBe(30);
    expect(demo.replay.state.reservations).toEqual({});
    expect(demo.replay.absentVersion).toBe(0);
    expect(demo.replay.appliedInOrder[0]).toContain('v1 inventory.stocked');
  });

  it('validates the reservations map: every value must be a finite number', () => {
    const payload = structuredClone(fullPayload) as Record<string, unknown>;
    const replay = recordOf(payload['replay'], 'replay');
    const state = recordOf(replay['state'], 'replay.state');
    state['reservations'] = { 'order-A': 20, 'order-B': 'many' };
    expect(() => archEventstoreDtoFromJson(payload)).toThrow('reservations["order-B"]');

    const withNumbers = structuredClone(fullPayload) as Record<string, unknown>;
    const replayNumbers = recordOf(withNumbers['replay'], 'replay');
    const stateNumbers = recordOf(replayNumbers['state'], 'replay.state');
    stateNumbers['reservations'] = { 'order-A': 20, 'order-B': 25 };
    expect(archEventstoreDtoFromJson(withNumbers).replay.state.reservations).toEqual({
      'order-A': 20,
      'order-B': 25,
    });
  });

  it('drops unknown keys (whitelist, not pass-through)', () => {
    const padded = { ...structuredClone(fullPayload), surprise: 'nope' } as Record<string, unknown>;
    const demo = archEventstoreDtoFromJson(padded) as unknown as Record<string, unknown>;
    expect(demo['surprise']).toBeUndefined();
  });

  it.each([
    ['missing panels', (p: Record<string, unknown>) => delete p['panels']],
    ['missing replay evidence', (p: Record<string, unknown>) => delete p['replay']],
    ['missing store surface', (p: Record<string, unknown>) => delete p['store']],
    [
      'non-boolean byteIdentical',
      (p: Record<string, unknown>) => {
        recordOf(p['replay'], 'replay')['byteIdentical'] = 'yes';
      },
    ],
    [
      'row without its observed version',
      (p: Record<string, unknown>) => {
        const panel = recordOf(arrayOf(p['panels'], 'panels')[0], 'panels[0]');
        const rows = arrayOf(panel['rows'], 'panels[0].rows');
        delete recordOf(rows[0], 'row 0')['version'];
      },
    ],
    [
      'replay evidence without its state',
      (p: Record<string, unknown>) => {
        delete recordOf(p['replay'], 'replay')['state'];
      },
    ],
  ])('throws on %s', (_name, mutate) => {
    const payload = structuredClone(fullPayload) as Record<string, unknown>;
    mutate(payload);
    expect(() => archEventstoreDtoFromJson(payload)).toThrow();
  });
});
