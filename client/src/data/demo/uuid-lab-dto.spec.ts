import { describe, expect, it } from 'vitest';
import { uuidLabDtoFromJson, toUUIDLabDemo } from './uuid-lab-dto';
import type { UUIDLabDemoData } from '../../domain/demo/uuid-lab';

/**
 * UUIDLabDTO spec — hostile-input discipline PLUS the values-vary
 * contract: the wire's uuid VALUES are fresh randomness per load, so the
 * validator (and these tests) check their PROPERTIES — canonical 36-char
 * 8-4-4-4-12 form, version-7 third group, RFC-variant fourth group —
 * never literals. Counts and static rows are deterministic and pinned.
 */

/** Property-valid UUIDv7 strings (NOT server literals — any
 * specification-conforming value must parse). */
const V1 = '0192ac15-0211-7001-90d1-d2d3d4d5d6d7';
const V2 = '0192ac15-0211-7002-8abc-f1f2f3f4f5f6';
const V3 = '0192ac15-0212-7fff-9aaa-000102030405';
const V4X = '0192ac15-0212-7ffe-b123-ffeeddccbbaa';

const wellFormed: UUIDLabDemoData = {
  generator: {
    name: 'UUIDv7, hand-rolled (RFC 9562 §5.7)',
    randomSource: 'crypto/rand — 12-bit counter seed + 62 random bits per id',
    description: '48-bit Unix milliseconds + version 7 + monotonic 12-bit counter',
  },
  layout: [
    { field: 'unix_ts_ms', bits: 48, contents: 'Unix epoch milliseconds, big-endian' },
    { field: 'ver', bits: 4, contents: '0b0111 — version 7' },
    { field: 'rand_a', bits: 12, contents: 'a per-millisecond counter' },
    { field: 'var', bits: 2, contents: '0b10 — the RFC 4122/9562 variant marker' },
    { field: 'rand_b', bits: 62, contents: 'crypto/rand — uniqueness' },
  ],
  anatomy: { value: V1, unixMs: 1700000000001, version: 7, variant: 'RFC 4122/9562 (0b10)', length: 36 },
  batch: {
    count: 4,
    generatedInOrder: [V1, V2, V3, V4X],
    sortedLexically: [V1, V2, V3, V4X],
    sortingMatchesGenerationOrder: true,
  },
  collision: { generated: 10000, distinct: 10000, collisionFree: true, randomnessBits: 74 },
  comparison: [
    {
      strategy: 'bigserial',
      storage: '8 bytes',
      indexLocality: 'perfect',
      urlGuessability: 'enumerable',
      uniquenessScope: "one table's sequence",
      notes: 'cheap and ordered, but ids leak volume and pace',
    },
    {
      strategy: 'uuidv7 (this generator)',
      storage: '16 bytes',
      indexLocality: 'good',
      urlGuessability: 'unguessable',
      uniquenessScope: 'global',
      notes: 'the default here',
    },
    {
      strategy: 'uuidv4 (gen_random_uuid())',
      storage: '16 bytes',
      indexLocality: 'worst',
      urlGuessability: 'unguessable',
      uniquenessScope: 'global',
      notes: 'PostgreSQL\u2019s built-in DEFAULT',
    },
  ],
};

describe('uuidLabDtoFromJson', () => {
  it('parses any property-valid UUIDv7 batch (values vary, shape never)', () => {
    expect(uuidLabDtoFromJson(wellFormed)).toEqual(wellFormed);

    // A different, equally valid batch: fresh values, same shape.
    const fresh = {
      ...wellFormed,
      anatomy: { ...wellFormed.anatomy, value: V2 },
      batch: {
        ...wellFormed.batch,
        generatedInOrder: [V2, V3],
        sortedLexically: [V2, V3],
        count: 2,
      },
    };
    expect(uuidLabDtoFromJson(fresh)).toEqual(fresh);
  });

  it.each([
    ['uuid v4 (wrong version nibble)', 'e7d60c9a-6d4e-4c1f-9db4-2f2c8f5a4f61'],
    ['uppercase hex (server emits lowercase)', V1.toUpperCase()],
    ['missing hyphen group', '0192ac150211-7001-90d1-d2d3d4d5d6d7'],
    ['variant bits 00', '0192ac15-0211-7001-1abc-f1f2f3f4f5f6'],
    ['35 characters', '0192ac15-0211-7001-90d1-d2d3d4d5d6d'],
  ])('rejects a batch member that is %s', (_label, bad) => {
    expect(() =>
      uuidLabDtoFromJson({
        ...wellFormed,
        batch: { ...wellFormed.batch, generatedInOrder: [bad], sortedLexically: [bad], count: 1 },
      }),
    ).toThrow();
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing generator', { ...wellFormed, generator: undefined }],
    ['a non-object generator', { ...wellFormed, generator: 'goose' }],
    ['missing layout', { ...wellFormed, layout: undefined }],
    ['a non-number layout bits', { ...wellFormed, layout: [{ field: 'f', bits: 'many', contents: 'c' }] }],
    ['missing anatomy', { ...wellFormed, anatomy: undefined }],
    ['a non-number anatomy version', { ...wellFormed, anatomy: { ...wellFormed.anatomy, version: 'seven' } }],
    ['missing batch', { ...wellFormed, batch: undefined }],
    ['missing sortedLexically', { ...wellFormed, batch: { ...wellFormed.batch, sortedLexically: undefined } }],
    ['a non-boolean sortedness flag', { ...wellFormed, batch: { ...wellFormed.batch, sortingMatchesGenerationOrder: 'yes' } }],
    ['missing collision', { ...wellFormed, collision: undefined }],
    ['missing comparison', { ...wellFormed, comparison: undefined }],
    ['a non-object comparison row', { ...wellFormed, comparison: ['bigserial'] }],
    ['missing comparison notes', { ...wellFormed, comparison: [{ strategy: 'bigserial', storage: '8 bytes', indexLocality: 'perfect', urlGuessability: 'enumerable', uniquenessScope: 'one table' }] }],
  ])('throws for %s', (_label, body) => {
    expect(() => uuidLabDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = uuidLabDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      anatomy: { ...wellFormed.anatomy, sneaky: true },
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toUUIDLabDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toUUIDLabDemo(wellFormed)).toBe(wellFormed);
  });
});
