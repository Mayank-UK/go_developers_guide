import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetMongoConsoleDemoUseCase,
  MONGO_CONSOLE_DEMO_USE_CASE,
} from '../../../domain/demo/get-mongo-console-demo-usecase';
import type {
  MongoBsonTypeRow,
  MongoConsoleDemoData,
  MongoExplainRow,
  MongoQueryBeat,
  MongoSeedReport,
} from '../../../domain/demo/mongo-console';

/**
 * MongoConsoleDemo — the mongodb topic's console (demo block id
 * `db:mongo-console`). It renders the payload of GET
 * /api/demo/db/mongo-console: the idempotent keyed seed (re-runs match,
 * never duplicate), the application-layer refusal (an invalid event
 * rejected with zero I/O), the query beats — deploy filter with sort
 * and limit, prod-tag containment, count, and the three-field
 * projection — the explain pair (COLLSCAN vs a hinted IXSCAN), the TTL
 * index's own spec as listIndexes states it, and the BSON type survey
 * of one stored document.
 *
 * An unreachable MongoDB degrades to the typed error panel (the
 * server's honest 503 — the optional-service discipline the topic's
 * second section teaches), never to a crash and never to a stale
 * answer.
 */

type MongoConsoleStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: MongoConsoleDemoData }
  | { state: 'error'; failure: Failure };

export function useMongoConsoleDemo(
  useCase: GetMongoConsoleDemoUseCase = container.resolve(MONGO_CONSOLE_DEMO_USE_CASE),
): { status: MongoConsoleStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<MongoConsoleStatus>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });

    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus({ state: 'ready', data: result.value });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function MongoConsoleDemo() {
  const { status, retry } = useMongoConsoleDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="mongo-console-loading" className="text-body-sm text-on-surface-variant">
        running the console…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the mongo console" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="mongo-console-ready">
      <SeedPanel seed={data.seed} database={data.database} collection={data.collection} />
      <RefusalPanel refusal={data.refusal} />

      {data.queries.map((beat) => (
        <QueryBeatPanel key={beat.op + beat.command} beat={beat} />
      ))}

      <section data-testid="mongo-console-explains" className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">Explain: COLLSCAN vs IXSCAN</h4>
        {data.explains.map((explain) => (
          <ExplainRow key={explain.label} explain={explain} />
        ))}
        <p className="text-caption text-on-surface-variant">
          The hint pins the indexed plan — on eight documents the planner may honestly prefer a scan.
        </p>
      </section>

      <section data-testid="mongo-console-ttl" className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">The TTL index, as the server states it</h4>
        <p data-testid="mongo-console-ttl-definition" className="font-mono text-caption text-on-surface">
          {data.ttl.indexName} · key {data.ttl.key} · expireAfterSeconds {data.ttl.expireAfterSeconds.toLocaleString('en-US')}
        </p>
        <p className="text-caption text-on-surface-variant">{data.ttl.sweeperNote}</p>
      </section>

      <section data-testid="mongo-console-bson" className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">BSON types, as the wire carried them</h4>
        {data.bsonTypes.map((row) => (
          <BsonRow key={row.field} row={row} />
        ))}
      </section>
    </div>
  );
}

function SeedPanel({ seed, database, collection }: { seed: MongoSeedReport; database: string; collection: string }) {
  return (
    <section data-testid="mongo-console-seed" className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm">
      <p className="font-mono text-caption text-primary">
        seed · {collection} in {database}
      </p>
      <p data-testid="mongo-console-seed-proof" className="font-mono text-caption text-on-surface">
        {seed.countBefore} already present · {seed.upserted} planted this run · {seed.totalAfter} total ·
        indexes: {seed.indexesEnsured.join(', ')}
      </p>
      <p className="text-caption text-on-surface-variant">
        Keyed upserts: a re-run matches the eight events instead of inserting — idempotency you can watch.
      </p>
    </section>
  );
}

function RefusalPanel({ refusal }: { refusal: MongoConsoleDemoData['refusal'] }) {
  return (
    <section data-testid="mongo-console-refusal" className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm">
      <p className="font-mono text-caption text-primary">the boundary refused first</p>
      {refusal.errors.map((error) => (
        <p key={error} className="font-mono text-caption text-error">
          {error}
        </p>
      ))}
      <p className="text-caption text-on-surface-variant">{refusal.note}</p>
    </section>
  );
}

function QueryBeatPanel({ beat }: { beat: MongoQueryBeat }) {
  return (
    <section
      data-testid={`mongo-console-beat-${beat.op}`}
      className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
    >
      <p className="font-mono text-caption text-on-surface">
        <span className="text-secondary">{beat.op}</span> — {beat.outcome}
      </p>
      <p className="font-mono text-caption text-on-surface-variant">{beat.command}</p>
      {beat.documents.map((document, index) => (
        <p key={index} className="font-mono text-caption text-on-surface-variant">
          {Object.entries(document)
            .map(([field, value]) => `${field}: ${value}`)
            .join(' · ')}
        </p>
      ))}
      <p className="text-caption text-on-surface-variant">{beat.description}</p>
    </section>
  );
}

function ExplainRow({ explain }: { explain: MongoExplainRow }) {
  const indexed = explain.stage === 'IXSCAN';
  return (
    <p data-testid="mongo-console-explain" className="font-mono text-caption text-on-surface">
      {explain.label}: <span className={indexed ? 'text-primary' : 'text-error'}>{explain.stage}</span>
      {indexed ? ` on ${explain.indexName}` : ''} · keys={explain.keysExamined} docs={explain.docsExamined}
    </p>
  );
}

function BsonRow({ row }: { row: MongoBsonTypeRow }) {
  return (
    <p className="font-mono text-caption text-on-surface-variant">
      <span className="text-on-surface">{row.field}</span> ({row.bsonType}) = {row.value} — {row.note}
    </p>
  );
}

export default MongoConsoleDemo;
