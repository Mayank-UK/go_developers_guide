import { describe, expect, it } from 'vitest';
import { archSagaDtoFromJson } from './arch-saga-dto';

/**
 * ArchSagaDTO spec — the fromJson whitelist: a full saga payload builds
 * the domain shape (the four scripted steps, the three runs with their
 * ledgers, world logs, counts, and the failure run's replay, the
 * semantics, the machinery surface), omitempty optionals decode as
 * undefined (a success run carries no trigger and no replay; empty is
 * DATA for the strings that can be empty — a non-compensatable step's
 * undo), and every violation (missing panels, wrong types, a non-number
 * seq) throws so parseDto can upgrade it to an InvalidResponseException.
 */

const fullPayload = {
  mode: 'compensation ledger: the real patterns.SagaRunner driven through three observed runs',
  source: 'server/internal/domain/patterns/saga.go',
  honestyNote: 'No-invention pin: every status, trigger, ledger row, and count was observed.',
  steps: [
    {
      order: 0,
      name: 'reserve-inventory',
      forward: 'inventory: reserved 3× gopher-plush for order o-1001',
      undo: 'inventory: released 3× gopher-plush for order o-1001',
    },
    {
      order: 3,
      name: 'send-confirmation',
      forward: 'email: confirmation queued for ops@example.com for order o-1001',
      undo: '',
    },
  ],
  runs: [
    {
      id: 'success',
      title: 'All four steps commit',
      lead: 'the happy path: every forward action runs in order, nothing is undone',
      script: 'four steps, no injected failure — the saga completes',
      status: 'completed',
      ledger: [
        { seq: 1, step: '0 · reserve-inventory', phase: 'forward', outcome: 'executed', detail: 'inventory: reserved 3× gopher-plush for order o-1001' },
      ],
      worldLog: ['inventory: reserved 3× gopher-plush for order o-1001'],
      counts: [{ label: 'steps executed', value: 1 }],
      note: 'the confirmation’s line is the last effect the world ever sees',
    },
    {
      id: 'failure',
      title: 'Step 2 fails: compensated in reverse',
      lead: 'book-shipment refuses; the charge and the reservation are undone, newest first',
      script: 'failure injected at step 2 — deterministic by index',
      status: 'compensated',
      trigger: 'saga stopped at step 2 (book-shipment): carrier: no ground pickup window for order o-1001',
      ledger: [
        { seq: 1, step: '0 · reserve-inventory', phase: 'forward', outcome: 'executed', detail: 'inventory: reserved 3× gopher-plush for order o-1001' },
        { seq: 2, step: '2 · book-shipment', phase: 'forward', outcome: 'failed', detail: 'carrier: no ground pickup window for order o-1001' },
      ],
      worldLog: [
        'inventory: reserved 3× gopher-plush for order o-1001',
        'payments: refunded 4200 cents to tok_visa_4242 for order o-1001',
      ],
      counts: [
        { label: 'already-compensated no-ops on the replay', value: 1 },
        { label: 'steps executed', value: 1 },
      ],
      replay: [
        { seq: 3, step: '1 · charge-card', phase: 'compensate', outcome: 'already-compensated', detail: 'compensating action not called again' },
      ],
      note: 'the replay drives Compensate a second time: no-ops, nothing called',
    },
  ],
  semantics: [
    'forward runs in order until a step fails or the context dies',
    'compensation runs in REVERSE over the steps that EXECUTED',
  ],
  surface: [
    { id: 'SagaRunner', kind: 'type', role: 'the orchestrator over ordered steps' },
    { id: 'SagaStatus', kind: 'vocabulary', role: 'completed | compensated | cancelled' },
  ],
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

describe('arch-saga-dto', () => {
  it('builds the domain shape from a full saga payload (whitelist)', () => {
    const demo = archSagaDtoFromJson(structuredClone(fullPayload));

    expect(demo.source).toBe('server/internal/domain/patterns/saga.go');
    expect(demo.steps[0]?.undo).toContain('released');
    // A non-compensatable step's undo is EMPTY and still present — empty is data.
    expect(demo.steps[1]?.undo).toBe('');

    const success = demo.runs[0];
    expect(success?.status).toBe('completed');
    // omitempty optionals decode as undefined: no trigger, no replay.
    expect(success?.trigger).toBeUndefined();
    expect(success?.replay).toBeUndefined();
    expect(success?.ledger[0]?.outcome).toBe('executed');
    expect(success?.worldLog[0]).toContain('reserved');

    const failure = demo.runs[1];
    expect(failure?.trigger).toContain('book-shipment');
    expect(failure?.replay?.[0]?.outcome).toBe('already-compensated');
    expect(failure?.counts[0]).toEqual({ label: 'already-compensated no-ops on the replay', value: 1 });

    expect(demo.semantics).toHaveLength(2);
    expect(demo.surface[1]?.kind).toBe('vocabulary');
  });

  it('drops unknown keys (whitelist, not pass-through)', () => {
    const padded = { ...structuredClone(fullPayload), surprise: 'nope' } as Record<string, unknown>;
    const demo = archSagaDtoFromJson(padded) as unknown as Record<string, unknown>;
    expect(demo['surprise']).toBeUndefined();
  });

  it.each([
    ['missing steps panel', (p: Record<string, unknown>) => delete p['steps']],
    ['missing runs', (p: Record<string, unknown>) => delete p['runs']],
    ['missing ledger inside a run', (p: Record<string, unknown>) => {
      const run = recordOf(arrayOf(p['runs'], 'runs')[0], 'runs[0]');
      delete run['ledger'];
    }],
    ['non-number seq in a ledger row', (p: Record<string, unknown>) => {
      const run = recordOf(arrayOf(p['runs'], 'runs')[0], 'runs[0]');
      const ledger = arrayOf(run['ledger'], 'runs[0].ledger');
      recordOf(ledger[0], 'ledger row 0')['seq'] = 'one';
    }],
    ['surface row without its role', (p: Record<string, unknown>) => {
      const surface = arrayOf(p['surface'], 'surface');
      delete recordOf(surface[0], 'surface row 0')['role'];
    }],
  ])('throws on %s', (_name, mutate) => {
    const payload = structuredClone(fullPayload) as Record<string, unknown>;
    mutate(payload);
    expect(() => archSagaDtoFromJson(payload)).toThrow();
  });

  it('still decodes when the failure run omits its replay (a run without one is legal)', () => {
    const payload = structuredClone(fullPayload) as Record<string, unknown>;
    const failure = recordOf(arrayOf(payload['runs'], 'runs')[1], 'runs[1]');
    delete failure['replay'];
    const demo = archSagaDtoFromJson(payload);
    expect(demo.runs[1]?.replay).toBeUndefined();
  });
});
