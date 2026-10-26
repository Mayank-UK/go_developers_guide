import type {
  MongoBsonTypeRow,
  MongoConsoleDemoData,
  MongoExplainRow,
  MongoQueryBeat,
  MongoRefusalRow,
  MongoSeedReport,
  MongoTtlShowcase,
} from '../../domain/demo/mongo-console';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * MongoConsoleDTO — the wire/transport shape of the mongo-console
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException.
 *
 * `indexName` is deliberately `requiredText` (present-but-empty passes):
 * a COLLSCAN row carries an empty index name, and an empty string there
 * is DATA, not absence. The same holds for refusal `attempted` values
 * like "(empty)". Counts are `requiredNumber` (zero is real: a re-run
 * upserts zero documents). Document rows are string maps — the server
 * renders every value as display text, so each value must be a string,
 * empty or not.
 */

export type MongoConsoleDemoDto = MongoConsoleDemoData;

/** Deserialize + validate the GET /api/demo/db/mongo-console payload. */
export function mongoConsoleDtoFromJson(body: unknown): MongoConsoleDemoDto {
  if (!isRecord(body)) throw new Error('mongo-console payload must be an object');
  return {
    database: requiredString(body, 'database'),
    collection: requiredString(body, 'collection'),
    seed: seedFromJson(requiredRecord(body, 'seed')),
    refusal: refusalFromJson(requiredRecord(body, 'refusal')),
    queries: requiredArray(body, 'queries').map(beatFromJson),
    explains: requiredArray(body, 'explains').map(explainFromJson),
    ttl: ttlFromJson(requiredRecord(body, 'ttl')),
    bsonTypes: requiredArray(body, 'bsonTypes').map(bsonRowFromJson),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toMongoConsoleDemo(dto: MongoConsoleDemoDto): MongoConsoleDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function seedFromJson(body: unknown): MongoSeedReport {
  if (!isRecord(body)) throw new Error('seed report must be an object');
  return {
    collection: requiredString(body, 'collection'),
    seedKeys: stringArray(body, 'seedKeys'),
    countBefore: requiredNumber(body, 'countBefore'),
    upserted: requiredNumber(body, 'upserted'),
    totalAfter: requiredNumber(body, 'totalAfter'),
    indexesEnsured: stringArray(body, 'indexesEnsured'),
  };
}

function refusalFromJson(body: unknown): MongoRefusalRow {
  if (!isRecord(body)) throw new Error('refusal row must be an object');
  return {
    attempted: stringMap(body, 'attempted'),
    errors: stringArray(body, 'errors'),
    note: requiredString(body, 'note'),
  };
}

function beatFromJson(body: unknown): MongoQueryBeat {
  if (!isRecord(body)) throw new Error('query beat must be an object');
  return {
    op: requiredString(body, 'op'),
    description: requiredString(body, 'description'),
    command: requiredString(body, 'command'),
    outcome: requiredString(body, 'outcome'),
    documents: requiredArray(body, 'documents').map(documentRow),
  };
}

function explainFromJson(body: unknown): MongoExplainRow {
  if (!isRecord(body)) throw new Error('explain row must be an object');
  return {
    label: requiredString(body, 'label'),
    query: requiredString(body, 'query'),
    stage: requiredString(body, 'stage'),
    indexName: requiredText(body, 'indexName'),
    keysExamined: requiredNumber(body, 'keysExamined'),
    docsExamined: requiredNumber(body, 'docsExamined'),
  };
}

function ttlFromJson(body: unknown): MongoTtlShowcase {
  if (!isRecord(body)) throw new Error('ttl showcase must be an object');
  return {
    collection: requiredString(body, 'collection'),
    indexName: requiredString(body, 'indexName'),
    key: requiredString(body, 'key'),
    expireAfterSeconds: requiredNumber(body, 'expireAfterSeconds'),
    sweeperNote: requiredString(body, 'sweeperNote'),
  };
}

function bsonRowFromJson(body: unknown): MongoBsonTypeRow {
  if (!isRecord(body)) throw new Error('bson type row must be an object');
  return {
    field: requiredString(body, 'field'),
    bsonType: requiredString(body, 'bsonType'),
    value: requiredText(body, 'value'),
    note: requiredString(body, 'note'),
  };
}

/** A string array whose entries must be non-empty (keys, index names). */
function stringArray(source: Record<string, unknown>, key: string): string[] {
  return requiredArray(source, key).map((entry) => {
    if (typeof entry !== 'string' || entry === '') {
      throw new Error(`expected a non-empty string at "${key}"`);
    }
    return entry;
  });
}

/** A field→value map of display strings; values may legitimately be empty. */
function stringMap(source: Record<string, unknown>, key: string): Record<string, string> {
  const raw = requiredRecord(source, key);
  const mapped: Record<string, string> = {};
  for (const [field, value] of Object.entries(raw)) {
    if (typeof value !== 'string') {
      throw new Error(`expected a string value at "${key}.${field}"`);
    }
    mapped[field] = value;
  }
  return mapped;
}

/** One document display row: every value a string, empty allowed. */
function documentRow(body: unknown): Record<string, string> {
  if (!isRecord(body) || Array.isArray(body)) throw new Error('document row must be an object');
  const mapped: Record<string, string> = {};
  for (const [field, value] of Object.entries(body)) {
    if (typeof value !== 'string') {
      throw new Error(`expected a string value at document.${field}`);
    }
    mapped[field] = value;
  }
  return mapped;
}
