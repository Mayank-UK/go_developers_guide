import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ContextCancellationApi } from './context-cancellation-api';
import { ContextCancellationRepositoryImpl } from './context-cancellation-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ContextCancellationRepositoryImpl spec — fixture-driven (happy +
 * failure): the happy payload (the REAL builder output, trimmed to its
 * load-bearing rows) round-trips into the domain shape; network /
 * envelope / malformed-payload failures all resolve as Failed Results,
 * never throws.
 */

const demoPayload = {
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
  workPhase: [
    { workerId: 'alpha', generation: 1, itemsProcessed: 2 },
    { workerId: 'gamma', generation: 2, itemsProcessed: 2 },
  ],
  propagation: [
    { workerId: 'alpha', generation: 1, derivedFrom: 'parent', observedPosition: 1, cause: 'context canceled' },
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

function makeRepository(): ContextCancellationRepositoryImpl {
  return new ContextCancellationRepositoryImpl(new ContextCancellationApi(createAppConfig()));
}

describe('ContextCancellationRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/conc/context', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.choreography.deadlineBudgetMs).toBe(400);
      expect(result.value.propagation[1]?.derivedFrom).toBe('alpha');
      expect(result.value.propagation[1]?.observedPosition).toBe(3);
      expect(result.value.deadlineBeat.deadlineExceeded).toBe(true);
      expect(result.value.ledger.outcome).toContain('no goroutines leaked');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });

  it('guards an error envelope into a Failed Result with its diagnostics', async () => {
    stubApiFetch([
      {
        url: '/api/demo/conc/context',
        status: 503,
        body: { error: { code: 'service_unavailable', message: 'calm down', requestId: 'req-9' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('calm down', {
          code: 'service_unavailable',
          status: 503,
          requestId: 'req-9',
        }),
      ),
    );
  });

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // Missing every beat — the DTO validator must reject it.
      { url: '/api/demo/conc/context', body: { propagation: 'nope' } },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('The server response did not match the expected shape.', {
          code: 'invalid_response',
          status: 200,
        }),
      ),
    );
  });

  it('rejects a non-number observed position instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/conc/context',
        body: {
          ...demoPayload,
          propagation: [{ ...demoPayload.propagation[0]!, observedPosition: 'first' }],
        },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });
});
