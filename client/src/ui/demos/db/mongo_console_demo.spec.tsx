import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MongoConsoleDemo } from './mongo_console_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetMongoConsoleDemoUseCase,
  MONGO_CONSOLE_DEMO_USE_CASE,
} from '../../../domain/demo/get-mongo-console-demo-usecase';
import type { MongoConsoleDemoData } from '../../../domain/demo/mongo-console';

/**
 * MongoConsoleDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the seed proof renders (idempotent replays visible as 0 planted), the
 * refusal renders, every query beat renders its command and documents,
 * the projection beat carries exactly its three fields, the explain
 * pair renders COLLSCAN and IXSCAN distinctly, the TTL spec renders,
 * the BSON survey renders — and failures, including the server's honest
 * 503 for an unreachable MongoDB, degrade to the shared error panel
 * with recovery.
 */

const fixture: MongoConsoleDemoData = {
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
    errors: [
      'key — lowercase kebab-case (got NOT A KEYPATTERN!)',
      'type — one of "deploy", "alert", "scaling", or "backup" (got restart)',
    ],
    note: 'Refused in the application layer — MongoDB never saw this event.',
  },
  queries: [
    {
      op: 'find',
      description: 'Filter, sort, limit: the compound index’s exact shape.',
      command: 'db.events.find({type: "deploy"}).sort({occurredAt: -1}).limit(2)',
      outcome: '2 of the deploy events, newest first',
      documents: [{ evtKey: 'deploy-api-0309', severity: 'info', occurredAt: '2026-03-09T08:55:00Z' }],
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
    { label: 'no index on severity', query: 'find({severity: "critical"})', stage: 'COLLSCAN', indexName: '', keysExamined: 0, docsExamined: 8 },
    { label: 'compound index, hinted', query: 'find({type: "deploy"})', stage: 'IXSCAN', indexName: 'type_1_occurredAt_-1', keysExamined: 3, docsExamined: 3 },
  ],
  ttl: {
    collection: 'events',
    indexName: 'occurredAt_1',
    key: '{occurredAt: 1}',
    expireAfterSeconds: 315360000,
    sweeperNote: 'MongoDB’s TTL monitor sweeps roughly every 60 seconds.',
  },
  bsonTypes: [
    { field: '_id', bsonType: 'objectId', value: '68a1c2d30000000000000007', note: 'assigned by the server at the first insert — re-upserts keep it stable' },
    { field: 'tags', bsonType: 'array', value: '[deploy prod]', note: 'one field holds the whole set — the containment query rides it' },
  ],
};

function stubUseCase(execute: () => Promise<Result<MongoConsoleDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    MONGO_CONSOLE_DEMO_USE_CASE,
    { execute } as unknown as GetMongoConsoleDemoUseCase,
  );
}

describe('MongoConsoleDemo', () => {
  it('renders loading first, then the whole console', async () => {
    stubUseCase(async () => success(fixture));

    render(<MongoConsoleDemo />);

    expect(screen.getByTestId('mongo-console-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('mongo-console-ready')).toBeInTheDocument());

    // The seed proof: a re-run plants nothing — idempotency visible.
    const seed = screen.getByTestId('mongo-console-seed');
    expect(seed).toHaveTextContent('8 already present');
    expect(seed).toHaveTextContent('0 planted this run');
    expect(seed).toHaveTextContent('evtKey_1');

    // The refusal, verbatim.
    const refusal = screen.getByTestId('mongo-console-refusal');
    expect(refusal).toHaveTextContent('lowercase kebab-case');
    expect(refusal).toHaveTextContent('MongoDB never saw this event');

    // The beats: commands and documents render.
    const find = screen.getByTestId('mongo-console-beat-find');
    expect(find).toHaveTextContent('db.events.find({type: "deploy"}).sort({occurredAt: -1}).limit(2)');
    expect(find).toHaveTextContent('evtKey: deploy-api-0309');
    const projected = screen.getByTestId('mongo-console-beat-find+project');
    expect(projected).toHaveTextContent('3 partial documents');

    // The explain pair, COLLSCAN and IXSCAN rendered distinctly.
    const explains = screen.getAllByTestId('mongo-console-explain');
    expect(explains).toHaveLength(2);
    expect(explains[0]).toHaveTextContent('COLLSCAN');
    expect(explains[1]).toHaveTextContent('IXSCAN on type_1_occurredAt_-1');

    // The TTL spec and the sweeper's cadence.
    const ttl = screen.getByTestId('mongo-console-ttl');
    expect(ttl).toHaveTextContent('occurredAt_1');
    expect(ttl).toHaveTextContent('315,360,000');

    // The BSON survey.
    const bson = screen.getByTestId('mongo-console-bson');
    expect(bson).toHaveTextContent('objectId');
    expect(bson).toHaveTextContent('re-upserts keep it stable');
  });

  it('degrades to the error panel on the server’s honest 503, with recovery', async () => {
    const failure = new ServerFailure(
      'The mongo console runs its beats against the guide’s live MongoDB, which is not reachable right now.',
      { code: 'service_unavailable', status: 503, requestId: 'req-503' },
    );
    stubUseCase(async () => failed(failure));

    render(<MongoConsoleDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');
    expect(panel).toHaveTextContent('not reachable right now');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('mongo-console-ready')).toBeInTheDocument());
  });
});
