import { describe, expect, it } from 'vitest';
import { MigrationsRepositoryImpl } from './db-migrations-repository-impl';
import type { MigrationsApi } from './db-migrations-api';
import type { MigrationsDemoData } from '../../domain/demo/db-migrations';

const fixture: MigrationsDemoData = { history: [], steps: [], summary: 's' };

describe('MigrationsRepositoryImpl', () => {
  it('wraps the api payload in a success Result', async () => {
    const repo = new MigrationsRepositoryImpl({ get: async () => fixture } as unknown as MigrationsApi);
    expect((await repo.get()).ok).toBe(true);
  });

  it('guards api rejections into failed Results', async () => {
    const repo = new MigrationsRepositoryImpl({
      get: async () => { throw new Error('boom'); },
    } as unknown as MigrationsApi);
    expect((await repo.get()).ok).toBe(false);
  });
});
