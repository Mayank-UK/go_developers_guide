import { describe, expect, it } from 'vitest';
import { RaceRepositoryImpl } from './db-race-repository-impl';
import type { RaceApi } from './db-race-api';
import type { RaceDemoData } from '../../domain/demo/db-race';

const fixture = {
  isolation: 'i',
  scratchBefore: 'b',
  scratchAfter: 'a',
  steps: [],
  lockWaited: true,
  serializableNote: 'n',
} as RaceDemoData;

describe('RaceRepositoryImpl', () => {
  it('wraps the api payload in a success Result', async () => {
    const repo = new RaceRepositoryImpl({ get: async () => fixture } as unknown as RaceApi);
    expect((await repo.get()).ok).toBe(true);
  });

  it('guards api rejections into failed Results', async () => {
    const repo = new RaceRepositoryImpl({
      get: async () => { throw new Error('boom'); },
    } as unknown as RaceApi);
    expect((await repo.get()).ok).toBe(false);
  });
});
