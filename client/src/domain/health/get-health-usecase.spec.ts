import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure, type Failure } from '../../core/error-handling/failures';
import type { HealthRepository } from './health-repository';
import type { HealthStatus } from './health-status';
import { GetHealthUseCase } from './get-health-usecase';

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(result: Result<HealthStatus>): HealthRepository {
  return {
    getHealth: () => Promise.resolve(result),
  };
}

describe('GetHealthUseCase', () => {
  it('passes a healthy repository result through as a Success', async () => {
    const useCase = new GetHealthUseCase(
      fakeRepository(success({ status: 'ok', env: 'dev' })),
    );

    await expect(useCase.execute()).resolves.toEqual({
      ok: true,
      value: { status: 'ok', env: 'dev' },
    });
  });

  it('passes a repository failure through as a Failed (never throws)', async () => {
    const failure: Failure = new NetworkFailure();
    const useCase = new GetHealthUseCase(fakeRepository(failed(failure)));

    await expect(useCase.execute()).resolves.toEqual(failed(failure));
  });
});
