/**
 * Domain models for the mongodb topic's console (demo block id
 * `db:mongo-console`) — the client-side shape of
 * GET /api/demo/db/mongo-console. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and domain
 * may diverge later).
 *
 * One payload carries the whole scripted run against the live MongoDB:
 * the idempotent keyed seed, the application-layer refusal (zero I/O),
 * the query beats (filter/sort/limit, tag containment, count,
 * projection), the COLLSCAN-vs-IXSCAN explain pair, the TTL index's own
 * spec as listIndexes states it, and the BSON type survey of one stored
 * document.
 */

/** The seed step: keyed upserts, so repeats are idempotent. */
export interface MongoSeedReport {
  collection: string;
  seedKeys: string[];
  countBefore: number;
  upserted: number;
  totalAfter: number;
  indexesEnsured: string[];
}

/** The boundary's refusal: what was attempted and why it never ran. */
export interface MongoRefusalRow {
  attempted: Record<string, string>;
  errors: string[];
  note: string;
}

/** One console beat and what the live MongoDB answered. */
export interface MongoQueryBeat {
  op: string;
  description: string;
  command: string;
  outcome: string;
  /** Display rows: field → rendered value (values are strings on the wire). */
  documents: Record<string, string>[];
}

/** One explain(): the winning plan's scan stage with its exam counts. */
export interface MongoExplainRow {
  label: string;
  query: string;
  stage: string;
  /** Empty string when the plan is a COLLSCAN — present-but-empty is data. */
  indexName: string;
  keysExamined: number;
  docsExamined: number;
}

/** The TTL exhibit: a real expireAfterSeconds index, spec read back. */
export interface MongoTtlShowcase {
  collection: string;
  indexName: string;
  key: string;
  expireAfterSeconds: number;
  sweeperNote: string;
}

/** One field of the BSON type survey: type, wire value, and the note. */
export interface MongoBsonTypeRow {
  field: string;
  bsonType: string;
  value: string;
  note: string;
}

/** The whole console payload: seed, refusal, beats, exhibits. */
export interface MongoConsoleDemoData {
  database: string;
  collection: string;
  seed: MongoSeedReport;
  refusal: MongoRefusalRow;
  queries: MongoQueryBeat[];
  explains: MongoExplainRow[];
  ttl: MongoTtlShowcase;
  bsonTypes: MongoBsonTypeRow[];
}
