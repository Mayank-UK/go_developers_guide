import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { ScenarioState } from './scenario';
import type { ScenarioRepository } from './scenario-repository';
import { GetScenarioUseCase } from './get-scenario-usecase';

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<ScenarioState>): ScenarioRepository {
  return {
    getScenario: () => Promise.resolve(get),
    patchScenario: () => {
      throw new Error('not expected in this use case');
    },
  };
}

const state: ScenarioState = { dataMode: 'fake', injectFailure: 'none' };

describe('GetScenarioUseCase', () => {
  it('returns the current scenario snapshot', async () => {
    const useCase = new GetScenarioUseCase(fakeRepository(success(state)));

    const result = await useCase.execute();

    expect(result).toEqual(success(state));
  });

  it('returns a Failed Result when the store is unreachable (never throws)', async () => {
    const useCase = new GetScenarioUseCase(
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
