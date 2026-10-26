/**
 * Domain models for the redis topic's playground (demo block id
 * `redis:playground`) — the client-side shape of
 * `GET /api/demo/redis/playground`. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and
 * domain may diverge later).
 *
 * One payload carries the whole scripted walk against the RUNNING stack
 * Redis: the five core structures with their real commands and
 * read-backs, the pipelined INCR counter with its exact total, the TTL
 * countdown read as data (plus the SET NX lock pair's two answers), one
 * real pub/sub round-trip reported from both sides, and the cleanup that
 * removes every key the walk created.
 */

/** One value a structure's read returned, as a display string. */
export interface RedisValueRow {
  label: string;
  value: string;
}

/** One Redis structure walked with real commands and real results. */
export interface RedisStructureRow {
  kind: 'string' | 'hash' | 'list' | 'set' | 'zset';
  key: string;
  description: string;
  commands: string[];
  outcome: string;
  values: RedisValueRow[];
}

/** The atomicity exhibit: N pipelined INCRs, one round trip, an exact total. */
export interface RedisCounterProof {
  key: string;
  pipelineSize: number;
  commands: string[];
  total: number;
  proof: string;
}

/** The expiry exhibit: the configured EX, the live countdown, the NX pair. */
export interface RedisTTLProof {
  key: string;
  ttlSeconds: number;
  commands: string[];
  ttlFirstRead: number;
  nxFirstAcquired: boolean;
  nxSecondAcquired: boolean;
  proof: string;
}

/** The pub/sub exhibit: one real message, both sides of the wire. */
export interface RedisPubSubProof {
  channel: string;
  payload: string;
  commands: string[];
  receivers: number;
  receivedPayload: string;
  matched: boolean;
  proof: string;
}

/** The no-residue exhibit: how many keys the closing SCAN+DEL removed. */
export interface RedisCleanupProof {
  keysRemoved: number;
  commands: string[];
  proof: string;
}

/** The whole playground payload. */
export interface RedisPlaygroundDemoData {
  namespace: string;
  structures: RedisStructureRow[];
  counter: RedisCounterProof;
  ttl: RedisTTLProof;
  pubsub: RedisPubSubProof;
  cleanup: RedisCleanupProof;
  reasoning: string;
}
