import type {
  CachingLabBeat,
  CachingLabDemoData,
  CachingLabHerd,
  CachingLabStats,
  CachingLabSwap,
} from '../../domain/demo/caching-lab';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
} from '../content/validate';

/**
 * CachingLabDTO — the wire/transport shape of the lab payload. Mirrors the
 * domain model 1:1; the `fromJson` builds ONLY the listed fields
 * (whitelist) and throws on any shape violation, which parseDto upgrades
 * to an InvalidResponseException. Beat outcomes validate against the
 * server's closed vocabulary so a drifting classifier fails here first.
 */

export type CachingLabDemoDto = CachingLabDemoData;

const BEAT_OUTCOMES: readonly string[] = ['miss', 'hit', 'invalidated', 'expired', 'not-found'];

function stringArray(source: Record<string, unknown>, key: string): string[] {
  return requiredArray(source, key).map((line) => {
    if (typeof line !== 'string' || line === '') {
      throw new Error(`expected a non-empty string at "${key}"`);
    }
    return line;
  });
}

function beatFromJson(value: unknown): CachingLabBeat {
  if (!isRecord(value)) throw new Error('lab beat must be an object');
  const outcome = requiredString(value, 'outcome');
  if (!BEAT_OUTCOMES.includes(outcome)) {
    throw new Error(`unknown beat outcome "${outcome}"`);
  }
  return {
    id: requiredString(value, 'id'),
    title: requiredString(value, 'title'),
    script: stringArray(value, 'script'),
    outcome: outcome as CachingLabBeat['outcome'],
    observedValue:
      typeof value.observedValue === 'string' ? value.observedValue : '',
    detail: requiredString(value, 'detail'),
    hits: requiredNumber(value, 'hits'),
    misses: requiredNumber(value, 'misses'),
    storeReads: requiredNumber(value, 'storeReads'),
  };
}

function herdFromJson(body: Record<string, unknown>): CachingLabHerd {
  const herd = requiredRecord(body, 'herd');
  return {
    id: requiredString(herd, 'id'),
    readers: requiredNumber(herd, 'readers'),
    storeReads: requiredNumber(herd, 'storeReads'),
    sameAnswer: requiredBoolean(herd, 'sameAnswer'),
    answer: requiredString(herd, 'answer'),
    proof: requiredString(herd, 'proof'),
  };
}

function swapFromJson(body: Record<string, unknown>): CachingLabSwap {
  const swap = requiredRecord(body, 'swap');
  return {
    liveOnlyId: requiredString(swap, 'liveOnlyId'),
    liveOnlyTitle: requiredString(swap, 'liveOnlyTitle'),
    notFoundId: requiredString(swap, 'notFoundId'),
    notFoundSurfaced: requiredBoolean(swap, 'notFoundSurfaced'),
    fakeTitle: requiredString(swap, 'fakeTitle'),
    liveTitle: requiredString(swap, 'liveTitle'),
    backTitle: requiredString(swap, 'backTitle'),
    proof: requiredString(swap, 'proof'),
  };
}

function statsFromJson(body: Record<string, unknown>): CachingLabStats {
  const stats = requiredRecord(body, 'stats');
  return {
    hits: requiredNumber(stats, 'hits'),
    misses: requiredNumber(stats, 'misses'),
    expiries: requiredNumber(stats, 'expiries'),
    invalidations: requiredNumber(stats, 'invalidations'),
    storeReads: requiredNumber(stats, 'storeReads'),
    entries: requiredNumber(stats, 'entries'),
    proof: requiredString(stats, 'proof'),
  };
}

/** Deserialize + validate the GET /api/demo/caching/lab payload. */
export function cachingLabDtoFromJson(body: unknown): CachingLabDemoDto {
  if (!isRecord(body)) throw new Error('caching lab payload must be an object');
  return {
    ttlSeconds: requiredNumber(body, 'ttlSeconds'),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    herd: herdFromJson(body),
    swap: swapFromJson(body),
    stats: statsFromJson(body),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toCachingLabDemo(dto: CachingLabDemoDto): CachingLabDemoData {
  return dto;
}
