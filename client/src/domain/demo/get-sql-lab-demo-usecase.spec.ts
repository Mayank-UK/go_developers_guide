import { describe, expect, it } from 'vitest';
import { failed, success } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import { GetSQLLabDemoUseCase } from './get-sql-lab-demo-usecase';
import type { SQLLabRepository } from './sql-lab-repository';
import type { SQLLabDemoData } from './sql-lab';

const fixture: SQLLabDemoData = { queries: [] };

describe('GetSQLLabDemoUseCase', () => {
  it('returns the repository payload on success', async () => {
    const useCase = new GetSQLLabDemoUseCase({
      get: () => Promise.resolve(success(fixture)),
    } as SQLLabRepository);
    const result = await useCase.execute();
    expect(result.ok && result.value.queries).toHaveLength(0);
  });

  it('passes failures through untouched', async () => {
    const useCase = new GetSQLLabDemoUseCase({
      get: () => Promise.resolve(failed(new NetworkFailure('down'))),
    } as SQLLabRepository);
    expect((await useCase.execute()).ok).toBe(false);
  });
});
