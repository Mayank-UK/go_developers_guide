/**
 * Domain models for the caching topic's lab (demo block id `caching:lab`)
 * — the client-side shape of `GET /api/demo/caching/lab`. Field names match
 * the server's camelCase wire contract 1:1 (the DTO is the seam where wire
 * and domain may diverge later).
 *
 * One payload carries the whole cache-aside lifecycle run through the REAL
 * decorator on throwaway stores: the beats (cold read, warm hit, cached
 * page, write invalidation, TTL expiry), the twelve-reader herd coalesced
 * into one store read, the fake/live store swap that stayed
 * partition-honest, and the final scoreboard — on a payload deterministic
 * enough that the response itself carries the ETag the topic teaches.
 */

/** One choreographed beat: the calls as run, the outcome observed, and the
 * cumulative scoreboard after the beat. */
export interface CachingLabBeat {
  id: string;
  title: string;
  script: string[];
  outcome: 'miss' | 'hit' | 'invalidated' | 'expired' | 'not-found';
  observedValue: string;
  detail: string;
  hits: number;
  misses: number;
  storeReads: number;
}

/** The stampede census: how many readers raced one cold key, how many store
 * reads that herd cost, and whether every reader left with the same answer. */
export interface CachingLabHerd {
  id: string;
  readers: number;
  storeReads: number;
  sameAnswer: boolean;
  answer: string;
  proof: string;
}

/** The store-swap census: two stores answering differently for the same
 * ids, with no cache leak in either direction. */
export interface CachingLabSwap {
  liveOnlyId: string;
  liveOnlyTitle: string;
  notFoundId: string;
  notFoundSurfaced: boolean;
  fakeTitle: string;
  liveTitle: string;
  backTitle: string;
  proof: string;
}

/** The decorator's final scoreboard as the lab reports it. */
export interface CachingLabStats {
  hits: number;
  misses: number;
  expiries: number;
  invalidations: number;
  storeReads: number;
  entries: number;
  proof: string;
}

/** The whole caching lab payload. */
export interface CachingLabDemoData {
  ttlSeconds: number;
  beats: CachingLabBeat[];
  herd: CachingLabHerd;
  swap: CachingLabSwap;
  stats: CachingLabStats;
  reasoning: string;
}
