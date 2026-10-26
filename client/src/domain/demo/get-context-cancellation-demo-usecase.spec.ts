import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { ContextCancellationDemoData } from './context-cancellation';
import type { ContextCancellationRepository } from './context-cancellation-repository';
import { GetContextCancellationDemoUseCase } from './get-context-cancellation-demo-usecase';

/**
 * GetContextCancellationDemoUseCase spec — the use case is a thin
 * pass-through seam, so the contract to pin is exactly that: whatever the
 * repository resolves (Success or Failed) arrives unchanged, and nothing
 * throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<ContextCancellationDemoData>): ContextCancellationRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: ContextCancellationDemoData = {
  choreography: {
    deadlineBudgetMs: 400,
    workItemsPerWorker: 2,
    trigger: 'explicit cancel of the parent, before the armed deadline',
    tree: [
      { from: 'parent', to: 'alpha' },
      { from: 'parent', to: 'beta' },
      { from: 'alpha', to: 'gamma' },
    ],
    ordering: 'recorded behind per-generation barriers, ids sorted within a generation',
  },
  workPhase: [{ workerId: 'alpha', generation: 1, itemsProcessed: 2 }],
  propagation: [
    { workerId: 'alpha', generation: 1, derivedFrom: 'parent', observedPosition: 1, cause: 'context canceled' },
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

describe('GetContextCancellationDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetContextCancellationDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetContextCancellationDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
