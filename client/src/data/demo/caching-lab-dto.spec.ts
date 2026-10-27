import { describe, expect, it } from 'vitest';
import { cachingLabDtoFromJson, toCachingLabDemo } from './caching-lab-dto';
import type { CachingLabDemoData } from '../../domain/demo/caching-lab';

/**
 * CachingLabDTO spec — whitelist parsing with shape violations throwing
 * (parseDto upgrades them to InvalidResponseException at the boundary).
 * Beat outcomes validate against the server's closed vocabulary; the
 * observedValue of the not-found beat is present-but-empty real data, so it
 * is accepted as an optional string rather than required non-empty.
 */

const wellFormed: CachingLabDemoData = {
  ttlSeconds: 5,
  beats: [
    {
      id: 'cold-read',
      title: 'The first read misses',
      script: ['GetByID("p-1")'],
      outcome: 'miss',
      observedValue: 'Why Go for backends',
      detail: 'The cache was cold: the store answered and the entry went in.',
      hits: 0,
      misses: 1,
      storeReads: 1,
    },
    {
      id: 'warm-read',
      title: 'The second read hits',
      script: ['GetByID("p-1")'],
      outcome: 'hit',
      observedValue: 'Why Go for backends',
      detail: 'Served from memory — the store-read counter did not move.',
      hits: 1,
      misses: 1,
      storeReads: 1,
    },
    {
      id: 'typed-miss',
      title: 'The typed miss surfaces',
      script: ['GetByID("p-7") // fake mode'],
      outcome: 'not-found',
      observedValue: '',
      detail: 'The store\u2019s typed miss surfaced untouched — errors are never cached.',
      hits: 9,
      misses: 10,
      storeReads: 10,
    },
  ],
  herd: {
    id: 'p-2',
    readers: 12,
    storeReads: 1,
    sameAnswer: true,
    answer: 'ServeMux patterns in Go 1.22',
    proof: '12 concurrent readers produced exactly 1 store read.',
  },
  swap: {
    liveOnlyId: 'p-7',
    liveOnlyTitle: 'A seventh post, live only',
    notFoundId: 'p-7',
    notFoundSurfaced: true,
    fakeTitle: 'Why Go for backends (edited by the lab)',
    liveTitle: 'Why Go for backends',
    backTitle: 'Why Go for backends (edited by the lab)',
    proof: 'the scenario\u2019s data mode is salted into every key.',
  },
  stats: {
    hits: 13,
    misses: 10,
    expiries: 2,
    invalidations: 2,
    storeReads: 10,
    entries: 5,
    proof: '10 misses and 10 store reads — equal by construction.',
  },
  reasoning: 'the response itself carries an ETag.',
};

describe('cachingLabDtoFromJson', () => {
  it('accepts a well-formed payload unchanged', () => {
    expect(cachingLabDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('accepts a missing observedValue (present-but-empty is real data on the not-found beat)', () => {
    const withoutValue = { ...wellFormed, beats: [{ ...wellFormed.beats[0] }] };
    delete (withoutValue.beats[0] as { observedValue?: string }).observedValue;

    const parsed = cachingLabDtoFromJson(withoutValue);

    expect(parsed.beats[0]?.observedValue).toBe('');
  });

  it('rejects an unknown beat outcome', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], outcome: 'maybe' }],
    };

    expect(() => cachingLabDtoFromJson(poisoned)).toThrow(/unknown beat outcome/);
  });

  it('rejects a beat whose script carries a non-string', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], script: ['GetByID("p-1")', 7] }],
    };

    expect(() => cachingLabDtoFromJson(poisoned)).toThrow(/script/);
  });

  it('rejects a payload missing the herd census', () => {
    const poisoned = { ...wellFormed } as Record<string, unknown>;
    delete poisoned.herd;

    expect(() => cachingLabDtoFromJson(poisoned)).toThrow(/herd/);
  });

  it('rejects a scoreboard missing a counter', () => {
    const poisoned = { ...wellFormed, stats: { ...wellFormed.stats } } as Record<string, unknown>;
    delete (poisoned.stats as Record<string, unknown>).expiries;

    expect(() => cachingLabDtoFromJson(poisoned)).toThrow(/expiries/);
  });

  it('rejects a non-object body outright', () => {
    expect(() => cachingLabDtoFromJson('nope')).toThrow(/must be an object/);
    expect(() => cachingLabDtoFromJson(null)).toThrow(/must be an object/);
  });

  it('toCachingLabDemo is the identity seam while DTO and domain are 1:1', () => {
    expect(toCachingLabDemo(wellFormed)).toBe(wellFormed);
  });
});
