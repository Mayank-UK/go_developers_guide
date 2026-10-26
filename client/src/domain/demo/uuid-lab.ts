/**
 * Domain models for the sql-schema topic's uuid lab — the client-side
 * shape of `GET /api/demo/db/uuid`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * IMPORTANT for consumers: counts, layout, and comparison rows are
 * deterministic, but the uuid VALUES (anatomy.value, both batch arrays)
 * are fresh randomness on every load — assert their PROPERTIES (canonical
 * 36-char form, sortedness, distinct counts), never their literals.
 */

/** What minted the batch: the guide's hand-rolled RFC 9562 generator. */
export interface UUIDGeneratorInfo {
  name: string;
  randomSource: string;
  description: string;
}

/** One field of the v7 bit layout, field by field. */
export interface UUIDLayoutRow {
  field: string;
  bits: number;
  contents: string;
}

/** One freshly minted value, dissected. */
export interface UUIDAnatomy {
  value: string;
  /** The Unix-millisecond timestamp recovered from the first 48 bits. */
  unixMs: number;
  version: number;
  variant: string;
  length: number;
}

/** The sortedness proof: a batch minted in order beside its sorted copy. */
export interface UUIDBatch {
  count: number;
  generatedInOrder: string[];
  sortedLexically: string[];
  /** Lexical order equals mint order — the btree-locality argument,
   * proven by comparison, not asserted. */
  sortingMatchesGenerationOrder: boolean;
}

/** The collision experiment at volume. */
export interface UUIDCollisionCheck {
  generated: number;
  distinct: number;
  collisionFree: boolean;
  randomnessBits: number;
}

/** One row of the id-strategy comparison table. */
export interface UUIDStrategyRow {
  strategy: string;
  storage: string;
  indexLocality: string;
  urlGuessability: string;
  uniquenessScope: string;
  notes: string;
}

export interface UUIDLabDemoData {
  generator: UUIDGeneratorInfo;
  layout: UUIDLayoutRow[];
  anatomy: UUIDAnatomy;
  batch: UUIDBatch;
  collision: UUIDCollisionCheck;
  comparison: UUIDStrategyRow[];
}
