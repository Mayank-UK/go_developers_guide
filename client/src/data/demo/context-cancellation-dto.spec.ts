import { describe, expect, it } from 'vitest';
import { contextCancellationDtoFromJson, toContextCancellationDemo } from './context-cancellation-dto';
import type { ContextCancellationDemoData } from '../../domain/demo/context-cancellation';

/**
 * ContextCancellationDTO spec — hostile-input discipline: only the exact
 * wire shape parses, anything else throws (and therefore surfaces
 * upstream as an invalid_response Failure, never as a guessed default).
 * The well-formed fixture is the REAL builder output of
 * GET /api/demo/conc/context (captured from the running builder), not a
 * hand-approximation.
 */

const wellFormed: ContextCancellationDemoData = {
  choreography: {
    deadlineBudgetMs: 400,
    workItemsPerWorker: 2,
    trigger: 'explicit cancel of the parent, before the armed deadline',
    tree: [
      { from: 'parent', to: 'alpha' },
      { from: 'parent', to: 'beta' },
      { from: 'alpha', to: 'gamma' },
    ],
    ordering:
      'recorded behind per-generation barriers, ids sorted within a generation — within one generation the scheduler picks; the guarantee is that every descendant observes Done()',
  },
  workPhase: [
    { workerId: 'alpha', generation: 1, itemsProcessed: 2 },
    { workerId: 'beta', generation: 1, itemsProcessed: 2 },
    { workerId: 'gamma', generation: 2, itemsProcessed: 2 },
  ],
  propagation: [
    { workerId: 'alpha', generation: 1, derivedFrom: 'parent', observedPosition: 1, cause: 'context canceled' },
    { workerId: 'beta', generation: 1, derivedFrom: 'parent', observedPosition: 2, cause: 'context canceled' },
    { workerId: 'gamma', generation: 2, derivedFrom: 'alpha', observedPosition: 3, cause: 'context canceled' },
  ],
  deadlineBeat: {
    budgetMs: 25,
    cancelCalled: false,
    outcome: 'deadline exceeded',
    cause: 'context deadline exceeded',
    deadlineExceeded: true,
  },
  ledger: {
    goroutinesLaunched: 4,
    goroutinesJoined: 4,
    contextsCreated: 5,
    contextsCancelled: 5,
    outcome: 'no goroutines leaked, every context cancelled',
  },
};

describe('contextCancellationDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(contextCancellationDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps the propagation order and causes exactly as recorded', () => {
    const parsed = contextCancellationDtoFromJson(wellFormed);

    expect(parsed.propagation.map((row) => row.workerId)).toEqual(['alpha', 'beta', 'gamma']);
    expect(parsed.propagation.map((row) => row.observedPosition)).toEqual([1, 2, 3]);
    expect(parsed.propagation[2]?.derivedFrom).toBe('alpha');
    for (const row of parsed.propagation) {
      expect(row.cause).toBe('context canceled');
    }
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing choreography', { ...wellFormed, choreography: undefined }],
    ['missing workPhase', { ...wellFormed, workPhase: undefined }],
    ['missing propagation', { ...wellFormed, propagation: undefined }],
    ['missing deadlineBeat', { ...wellFormed, deadlineBeat: undefined }],
    ['missing ledger', { ...wellFormed, ledger: undefined }],
    ['propagation not an array', { ...wellFormed, propagation: 'nope' }],
    ['a non-object propagation row', { ...wellFormed, propagation: ['nope'] }],
    ['missing observedPosition', { ...wellFormed, propagation: [{ ...wellFormed.propagation[0]!, observedPosition: undefined }] }],
    ['NaN generation', { ...wellFormed, propagation: [{ ...wellFormed.propagation[0]!, generation: Number.NaN }] }],
    ['missing tree', { ...wellFormed, choreography: { ...wellFormed.choreography, tree: undefined } }],
    ['a non-object tree edge', { ...wellFormed, choreography: { ...wellFormed.choreography, tree: ['nope'] } }],
    ['missing cancelCalled', { ...wellFormed, deadlineBeat: { ...wellFormed.deadlineBeat, cancelCalled: undefined } }],
    ['non-boolean deadlineExceeded', { ...wellFormed, deadlineBeat: { ...wellFormed.deadlineBeat, deadlineExceeded: 'yes' } }],
    ['empty ledger outcome', { ...wellFormed, ledger: { ...wellFormed.ledger, outcome: '' } }],
    ['non-object choreography', { ...wellFormed, choreography: 'fixed' }],
  ])('throws for %s', (_label, body) => {
    expect(() => contextCancellationDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = contextCancellationDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      deadlineBeat: { ...wellFormed.deadlineBeat, sneaky: true },
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toContextCancellationDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toContextCancellationDemo(wellFormed)).toBe(wellFormed);
  });
});
