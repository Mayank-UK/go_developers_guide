import { describe, expect, it } from 'vitest';
import { MongoConsoleRepositoryImpl } from './mongo-console-repository-impl';
import type { MongoConsoleApi } from './mongo-console-api';
import type { MongoConsoleDemoData } from '../../domain/demo/mongo-console';

const fixture = {
  database: 'guide_demos',
  collection: 'events',
  seed: { collection: 'events', seedKeys: [], countBefore: 0, upserted: 8, totalAfter: 8, indexesEnsured: [] },
  refusal: { attempted: {}, errors: [], note: 'refused' },
  queries: [],
  explains: [],
  ttl: { collection: 'events', indexName: 'occurredAt_1', key: '{occurredAt: 1}', expireAfterSeconds: 315360000, sweeperNote: 'sweeps' },
  bsonTypes: [],
} as MongoConsoleDemoData;

describe('MongoConsoleRepositoryImpl', () => {
  it('wraps the api payload in a success Result', async () => {
    const repo = new MongoConsoleRepositoryImpl({ get: async () => fixture } as unknown as MongoConsoleApi);
    expect((await repo.get()).ok).toBe(true);
  });

  it('guards api rejections into failed Results', async () => {
    const repo = new MongoConsoleRepositoryImpl({
      get: async () => { throw new Error('boom'); },
    } as unknown as MongoConsoleApi);
    expect((await repo.get()).ok).toBe(false);
  });
});
