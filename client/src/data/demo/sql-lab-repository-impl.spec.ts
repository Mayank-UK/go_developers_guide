import { describe, expect, it } from 'vitest';
import { SQLLabRepositoryImpl } from './sql-lab-repository-impl';
import type { SQLLabApi } from './sql-lab-api';
import type { SQLLabDemoData } from '../../domain/demo/sql-lab';

const fixture: SQLLabDemoData = { queries: [] };

describe('SQLLabRepositoryImpl', () => {
  it('wraps the api payload in a success Result', async () => {
    const repo = new SQLLabRepositoryImpl({ get: async () => fixture } as unknown as SQLLabApi);
    expect((await repo.get()).ok).toBe(true);
  });

  it('guards api rejections into failed Results', async () => {
    const repo = new SQLLabRepositoryImpl({
      get: async () => { throw new Error('boom'); },
    } as unknown as SQLLabApi);
    expect((await repo.get()).ok).toBe(false);
  });
});
