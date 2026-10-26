import type {
  UUIDAnatomy,
  UUIDBatch,
  UUIDCollisionCheck,
  UUIDGeneratorInfo,
  UUIDLabDemoData,
  UUIDLayoutRow,
  UUIDStrategyRow,
} from '../../domain/demo/uuid-lab';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
} from '../content/validate';

/**
 * UUIDLabDTO — the wire/transport shape of the UUIDv7 lab payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException.
 *
 * VALUES-VARY CONTRACT (this is the demo's headline, so it is pinned
 * here): `anatomy.value` and both batch arrays carry FRESH randomness on
 * every load — specs and views must assert their properties (canonical
 * 36-char 8-4-4-4-12 form, version-7 third group, sortedness flag, the
 * batch's own member-for-member comparison), never literal uuid values.
 * The counts (batch 8, collision volume 10000) and every static row are
 * deterministic.
 */

export type UUIDLabDemoDto = UUIDLabDemoData;

/** Deserialize + validate the GET /api/demo/db/uuid payload. */
export function uuidLabDtoFromJson(body: unknown): UUIDLabDemoDto {
  if (!isRecord(body)) throw new Error('uuid-lab payload must be an object');
  return {
    generator: generatorFromJson(requiredRecordOrThrow(body, 'generator')),
    layout: requiredArray(body, 'layout').map(layoutRowFromJson),
    anatomy: anatomyFromJson(requiredRecordOrThrow(body, 'anatomy')),
    batch: batchFromJson(requiredRecordOrThrow(body, 'batch')),
    collision: collisionFromJson(requiredRecordOrThrow(body, 'collision')),
    comparison: requiredArray(body, 'comparison').map(strategyRowFromJson),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toUUIDLabDemo(dto: UUIDLabDemoDto): UUIDLabDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function requiredRecordOrThrow(source: Record<string, unknown>, key: string): Record<string, unknown> {
  const value = source[key];
  if (!isRecord(value)) throw new Error(`expected object at "${key}"`);
  return value;
}

function generatorFromJson(body: Record<string, unknown>): UUIDGeneratorInfo {
  return {
    name: requiredString(body, 'name'),
    randomSource: requiredString(body, 'randomSource'),
    description: requiredString(body, 'description'),
  };
}

function layoutRowFromJson(body: unknown): UUIDLayoutRow {
  if (!isRecord(body)) throw new Error('layout row must be an object');
  return {
    field: requiredString(body, 'field'),
    bits: requiredNumber(body, 'bits'),
    contents: requiredString(body, 'contents'),
  };
}

function anatomyFromJson(body: Record<string, unknown>): UUIDAnatomy {
  return {
    value: requiredString(body, 'value'),
    unixMs: requiredNumber(body, 'unixMs'),
    version: requiredNumber(body, 'version'),
    variant: requiredString(body, 'variant'),
    length: requiredNumber(body, 'length'),
  };
}

function batchFromJson(body: Record<string, unknown>): UUIDBatch {
  const generated = requiredArray(body, 'generatedInOrder');
  const sorted = requiredArray(body, 'sortedLexically');
  for (const value of [...generated, ...sorted]) {
    if (typeof value !== 'string' || !/^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(value)) {
      // Property check, not a literal: canonical form + version 7 +
      // RFC variant — the same properties the domain teaches.
      throw new Error('batch member is not a canonical RFC 9562 UUIDv7 string');
    }
  }
  return {
    count: requiredNumber(body, 'count'),
    generatedInOrder: generated as string[],
    sortedLexically: sorted as string[],
    sortingMatchesGenerationOrder: requiredBoolean(body, 'sortingMatchesGenerationOrder'),
  };
}

function collisionFromJson(body: Record<string, unknown>): UUIDCollisionCheck {
  return {
    generated: requiredNumber(body, 'generated'),
    distinct: requiredNumber(body, 'distinct'),
    collisionFree: requiredBoolean(body, 'collisionFree'),
    randomnessBits: requiredNumber(body, 'randomnessBits'),
  };
}

function strategyRowFromJson(body: unknown): UUIDStrategyRow {
  if (!isRecord(body)) throw new Error('strategy row must be an object');
  return {
    strategy: requiredString(body, 'strategy'),
    storage: requiredString(body, 'storage'),
    indexLocality: requiredString(body, 'indexLocality'),
    urlGuessability: requiredString(body, 'urlGuessability'),
    uniquenessScope: requiredString(body, 'uniquenessScope'),
    notes: requiredString(body, 'notes'),
  };
}
