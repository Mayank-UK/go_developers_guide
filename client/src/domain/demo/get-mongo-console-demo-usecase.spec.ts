import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { MongoConsoleDemoData } from './mongo-console';
import type { MongoConsoleRepository } from './mongo-console-repository';
import { GetMongoConsoleDemoUseCase } from './get-mongo-console-demo-usecase';

/**
 * GetMongoConsoleDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

function fakeRepository(get: Result<MongoConsoleDemoData>): MongoConsoleRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: MongoConsoleDemoData = {
  database: 'guide_demos',
  collection: 'events',
  seed: {
    collection: 'events',
    seedKeys: ['alert-disk-0221', 'deploy-api-0126'],
    countBefore: 8,
    upserted: 0,
    totalAfter: 8,
    indexesEnsured: ['evtKey_1', 'occurredAt_1', 'type_1_occurredAt_-1'],
  },
  refusal: {
    attempted: { key: 'NOT A KEYPATTERN!', type: 'restart' },
    errors: ['key — lowercase kebab-case (got NOT A KEYPATTERN!)'],
    note: 'Refused in the application layer — MongoDB never saw this event.',
  },
  queries: [
    {
      op: 'find',
      description: 'Filter, sort, limit.',
      command: 'db.events.find({type: "deploy"}).sort({occurredAt: -1}).limit(2)',
      outcome: '2 of the deploy events, newest first',
      documents: [{ evtKey: 'deploy-api-0309', severity: 'info' }],
    },
  ],
  explains: [
    { label: 'no index on severity', query: 'find({severity: "critical"})', stage: 'COLLSCAN', indexName: '', keysExamined: 0, docsExamined: 8 },
    { label: 'compound index, hinted', query: 'find({type: "deploy"})', stage: 'IXSCAN', indexName: 'type_1_occurredAt_-1', keysExamined: 3, docsExamined: 3 },
  ],
  ttl: {
    collection: 'events',
    indexName: 'occurredAt_1',
    key: '{occurredAt: 1}',
    expireAfterSeconds: 315360000,
    sweeperNote: 'MongoDB\u2019s TTL monitor sweeps roughly every 60 seconds.',
  },
  bsonTypes: [
    { field: '_id', bsonType: 'objectId', value: '68a1c2d30000000000000000', note: 'assigned by the server at the first insert' },
  ],
};

describe('GetMongoConsoleDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetMongoConsoleDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetMongoConsoleDemoUseCase(
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
