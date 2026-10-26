import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { ServerFailure } from '../../core/error-handling/failures';
import type { ScenarioPatch, ScenarioState } from './scenario';
import type { ScenarioRepository } from './scenario-repository';
import { PatchScenarioUseCase } from './patch-scenario-usecase';

/**
 * In-memory repository stand-in — the port's contract is all the use case
 * sees, including the exact patch it was handed (partial updates must pass
 * through untouched, never merged or defaulted here).
 */
function fakeRepository(
  patchResult: Result<ScenarioState>,
  seen: ScenarioPatch[] = [],
): ScenarioRepository {
  return {
    getScenario: () => {
      throw new Error('not expected in this use case');
    },
    patchScenario: (patch: ScenarioPatch) => {
      seen.push(patch);
      return Promise.resolve(patchResult);
    },
  };
}

const updated: ScenarioState = { dataMode: 'fake', injectFailure: 'latency' };

describe('PatchScenarioUseCase', () => {
  it('forwards the exact partial patch and returns the resulting state', async () => {
    const seen: ScenarioPatch[] = [];
    const useCase = new PatchScenarioUseCase(fakeRepository(success(updated), seen));

    const result = await useCase.execute({ injectFailure: 'latency' });

    expect(result).toEqual(success(updated));
    expect(seen).toEqual([{ injectFailure: 'latency' }]);
  });

  it('returns a Failed Result for an invalid value (the server 422), never throws', async () => {
    const useCase = new PatchScenarioUseCase(
      fakeRepository(
        failed(
          new ServerFailure('Invalid scenario patch: unknown failure injection.', {
            code: 'validation_failed',
            status: 422,
            requestId: 'req-9',
          }),
        ),
      ),
    );

    const result = await useCase.execute({ injectFailure: 'bogus' as never });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('validation_failed');
      expect(result.failure.status).toBe(422);
      expect(result.failure.requestId).toBe('req-9');
    }
  });
});
