import { describe, expect, it } from 'vitest';
import { mongoConsoleDtoFromJson } from './mongo-console-dto';

/**
 * mongoConsoleDtoFromJson spec — the fixture mirrors the REAL payload
 * GET /api/demo/db/mongo-console serves (seed/refusal/queries/explains/
 * ttl/bsonTypes, camelCase, counts as numbers, documents as string
 * maps). The pins: a well-formed payload builds the DTO; an empty
 * indexName on a COLLSCAN row is DATA and passes; zero counts pass;
 * shape violations throw.
 */

const payload = {
  database: 'guide_demos',
  collection: 'events',
  seed: {
    collection: 'events',
    seedKeys: ['alert-disk-0221', 'deploy-api-0126', 'deploy-api-0309'],
    countBefore: 8,
    upserted: 0,
    totalAfter: 8,
    indexesEnsured: ['evtKey_1', 'occurredAt_1', 'type_1_occurredAt_-1'],
  },
  refusal: {
    attempted: { key: 'NOT A KEYPATTERN!', severity: 'sev-1', source: '(empty)' },
    errors: [
      'key — lowercase kebab-case (got NOT A KEYPATTERN!)',
      'type — one of "deploy", "alert", "scaling", or "backup" (got restart)',
    ],
    note: 'Refused in the application layer — MongoDB never saw this event.',
  },
  queries: [
    {
      op: 'find',
      description: 'Filter, sort, limit.',
      command: 'db.events.find({type: "deploy"}).sort({occurredAt: -1}).limit(2)',
      outcome: '2 of the deploy events, newest first',
      documents: [
        { _id: '68a1c2d30000000000000007', evtKey: 'deploy-api-0309', severity: 'info', tags: '[deploy prod]', occurredAt: '2026-03-09T08:55:00Z' },
      ],
    },
    {
      op: 'find+project',
      description: 'Projection: three fields, _id excluded.',
      command: 'db.events.find({type: "deploy"}, {evtKey: 1, severity: 1, occurredAt: 1, _id: 0})',
      outcome: '3 partial documents — no _id, no tags, no message',
      documents: [{ evtKey: 'deploy-api-0309', severity: 'info', occurredAt: '2026-03-09T08:55:00Z' }],
    },
  ],
  explains: [
    { label: 'no index on severity — the planner scans', query: 'find({severity: "critical"})', stage: 'COLLSCAN', indexName: '', keysExamined: 0, docsExamined: 8 },
    { label: 'compound index, hinted — the planner walks the index', query: 'find({type: "deploy"}).sort({occurredAt: -1})', stage: 'IXSCAN', indexName: 'type_1_occurredAt_-1', keysExamined: 3, docsExamined: 3 },
  ],
  ttl: {
    collection: 'events',
    indexName: 'occurredAt_1',
    key: '{occurredAt: 1}',
    expireAfterSeconds: 315360000,
    sweeperNote: 'MongoDB’s TTL monitor sweeps roughly every 60 seconds; nothing vanishes mid-demo.',
  },
  bsonTypes: [
    { field: '_id', bsonType: 'objectId', value: '68a1c2d30000000000000007', note: 'assigned by the server at the first insert — re-upserts keep it stable' },
    { field: 'tags', bsonType: 'array', value: '[deploy prod]', note: 'one field holds the whole set' },
  ],
};

describe('mongoConsoleDtoFromJson', () => {
  it('builds the DTO from a well-formed payload', () => {
    const dto = mongoConsoleDtoFromJson(payload);
    expect(dto.database).toBe('guide_demos');
    expect(dto.seed.totalAfter).toBe(8);
    expect(dto.refusal.errors).toHaveLength(2);
    expect(dto.queries[1]?.documents[0]).toEqual({ evtKey: 'deploy-api-0309', severity: 'info', occurredAt: '2026-03-09T08:55:00Z' });
    expect(dto.explains[1]?.indexName).toBe('type_1_occurredAt_-1');
    expect(dto.ttl.expireAfterSeconds).toBe(315360000);
    expect(dto.bsonTypes[0]?.bsonType).toBe('objectId');
  });

  it('treats an empty indexName as data, not absence (COLLSCAN rows)', () => {
    const dto = mongoConsoleDtoFromJson(payload);
    expect(dto.explains[0]?.stage).toBe('COLLSCAN');
    expect(dto.explains[0]?.indexName).toBe('');
  });

  it('treats zero counts as data (a re-run upserts zero documents)', () => {
    const dto = mongoConsoleDtoFromJson({ ...payload, seed: { ...payload.seed, countBefore: 8, upserted: 0 } });
    expect(dto.seed.upserted).toBe(0);
  });

  it('refuses malformed payloads', () => {
    expect(() => mongoConsoleDtoFromJson([])).toThrow();
    expect(() => mongoConsoleDtoFromJson({ ...payload, seed: { ...payload.seed, upserted: '0' } })).toThrow();
    expect(() => mongoConsoleDtoFromJson({ ...payload, explains: [{ ...payload.explains[0]!, indexName: 3 }] })).toThrow();
    expect(() => mongoConsoleDtoFromJson({ ...payload, queries: [{ ...payload.queries[0]!, documents: [{ evtKey: 7 }] }] })).toThrow();
    expect(() => mongoConsoleDtoFromJson({ ...payload, database: '' })).toThrow();
  });
});
