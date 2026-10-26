import { describe, expect, it } from 'vitest';
import { SQLSafetyRepositoryImpl } from './sql-safety-repository-impl';
import type { SQLSafetyApi } from './sql-safety-api';
import type { SQLSafetyDemoData } from '../../domain/demo/sql-safety';

const fixture: SQLSafetyDemoData = {
  attacks: [],
  summary: 's',
};

describe('SQLSafetyRepositoryImpl', () => {
  it('wraps the api payload in a success Result', async () => {
    const api = { get: async () => fixture } as unknown as SQLSafetyApi;
    const repo = new SQLSafetyRepositoryImpl(api);
    const result = await repo.get();
    expect(result.ok).toBe(true);
  });

  it('guards api rejections into failed Results', async () => {
    const api = { get: async () => { throw new Error('boom'); } } as unknown as SQLSafetyApi;
    const repo = new SQLSafetyRepositoryImpl(api);
    const result = await repo.get();
    expect(result.ok).toBe(false);
  });
});
