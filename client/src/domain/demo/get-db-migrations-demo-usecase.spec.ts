import { describe, expect, it } from 'vitest';
import { failed, success } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import { GetMigrationsDemoUseCase } from './get-db-migrations-demo-usecase';
import type { MigrationsRepository } from './db-migrations-repository';
import type { MigrationsDemoData } from './db-migrations';

const fixture: MigrationsDemoData = { history: [], steps: [], summary: 's' };

describe('GetMigrationsDemoUseCase', () => {
  it('returns the repository payload on success', async () => {
    const useCase = new GetMigrationsDemoUseCase({
      get: () => Promise.resolve(success(fixture)),
    } as MigrationsRepository);
    const result = await useCase.execute();
    expect(result.ok && result.value.summary).toBe('s');
  });

  it('passes failures through untouched', async () => {
    const useCase = new GetMigrationsDemoUseCase({
      get: () => Promise.resolve(failed(new NetworkFailure('down'))),
    } as MigrationsRepository);
    expect((await useCase.execute()).ok).toBe(false);
  });
});
