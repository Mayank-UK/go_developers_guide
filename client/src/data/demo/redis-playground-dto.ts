import type {
  RedisCleanupProof,
  RedisCounterProof,
  RedisPlaygroundDemoData,
  RedisPubSubProof,
  RedisStructureRow,
  RedisTTLProof,
  RedisValueRow,
} from '../../domain/demo/redis-playground';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
} from '../content/validate';

/**
 * RedisPlaygroundDTO — the wire/transport shape of the playground
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException. Structure kinds
 * validate against the server's closed vocabulary so a drifting
 * classifier fails here first; the TTL countdown (`ttlFirstRead`) varies
 * with the store's live clock by design — asserted by range in the UI's
 * rendering, never pinned to a literal here.
 */

export type RedisPlaygroundDemoDto = RedisPlaygroundDemoData;

const STRUCTURE_KINDS: readonly string[] = ['string', 'hash', 'list', 'set', 'zset'];

function stringArray(source: Record<string, unknown>, key: string): string[] {
  return requiredArray(source, key).map((command) => {
    if (typeof command !== 'string' || command === '') {
      throw new Error(`expected a non-empty string at "${key}"`);
    }
    return command;
  });
}

function valueFromJson(value: unknown): RedisValueRow {
  if (!isRecord(value)) throw new Error('structure value row must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredString(value, 'value'),
  };
}

function structureFromJson(value: unknown): RedisStructureRow {
  if (!isRecord(value)) throw new Error('structure row must be an object');
  const kind = requiredString(value, 'kind');
  if (!STRUCTURE_KINDS.includes(kind)) {
    throw new Error(`unknown structure kind "${kind}"`);
  }
  return {
    kind: kind as RedisStructureRow['kind'],
    key: requiredString(value, 'key'),
    description: requiredString(value, 'description'),
    commands: stringArray(value, 'commands'),
    outcome: requiredString(value, 'outcome'),
    values: requiredArray(value, 'values').map(valueFromJson),
  };
}

function counterFromJson(body: Record<string, unknown>): RedisCounterProof {
  const counter = requiredRecord(body, 'counter');
  return {
    key: requiredString(counter, 'key'),
    pipelineSize: requiredNumber(counter, 'pipelineSize'),
    commands: stringArray(counter, 'commands'),
    total: requiredNumber(counter, 'total'),
    proof: requiredString(counter, 'proof'),
  };
}

function ttlFromJson(body: Record<string, unknown>): RedisTTLProof {
  const ttl = requiredRecord(body, 'ttl');
  return {
    key: requiredString(ttl, 'key'),
    ttlSeconds: requiredNumber(ttl, 'ttlSeconds'),
    commands: stringArray(ttl, 'commands'),
    ttlFirstRead: requiredNumber(ttl, 'ttlFirstRead'),
    nxFirstAcquired: requiredBoolean(ttl, 'nxFirstAcquired'),
    nxSecondAcquired: requiredBoolean(ttl, 'nxSecondAcquired'),
    proof: requiredString(ttl, 'proof'),
  };
}

function pubsubFromJson(body: Record<string, unknown>): RedisPubSubProof {
  const pubsub = requiredRecord(body, 'pubsub');
  return {
    channel: requiredString(pubsub, 'channel'),
    payload: requiredString(pubsub, 'payload'),
    commands: stringArray(pubsub, 'commands'),
    receivers: requiredNumber(pubsub, 'receivers'),
    receivedPayload: requiredString(pubsub, 'receivedPayload'),
    matched: requiredBoolean(pubsub, 'matched'),
    proof: requiredString(pubsub, 'proof'),
  };
}

function cleanupFromJson(body: Record<string, unknown>): RedisCleanupProof {
  const cleanup = requiredRecord(body, 'cleanup');
  return {
    keysRemoved: requiredNumber(cleanup, 'keysRemoved'),
    commands: stringArray(cleanup, 'commands'),
    proof: requiredString(cleanup, 'proof'),
  };
}

/** Deserialize + validate the GET /api/demo/redis/playground payload. */
export function redisPlaygroundDtoFromJson(body: unknown): RedisPlaygroundDemoDto {
  if (!isRecord(body)) throw new Error('redis playground payload must be an object');
  return {
    namespace: requiredString(body, 'namespace'),
    structures: requiredArray(body, 'structures').map(structureFromJson),
    counter: counterFromJson(body),
    ttl: ttlFromJson(body),
    pubsub: pubsubFromJson(body),
    cleanup: cleanupFromJson(body),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toRedisPlaygroundDemo(dto: RedisPlaygroundDemoDto): RedisPlaygroundDemoData {
  return dto;
}
