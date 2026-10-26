import type {
  SchemaExplorerColumn,
  SchemaExplorerConstraint,
  SchemaExplorerDemoData,
  SchemaExplorerHistory,
  SchemaExplorerIndex,
  SchemaExplorerNote,
  SchemaExplorerTable,
} from '../../domain/demo/schema-explorer';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * SchemaExplorerDTO — the wire/transport shape of the live-schema
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException.
 *
 * `purpose` and column `default` are deliberately `requiredText`
 * (present-but-empty passes): an uncurated table's empty purpose and a
 * column's absent default are DATA, not absence. Constraint kinds
 * validate against the server's closed vocabulary (plus OTHER for
 * catalog kinds outside the four taught ones) so a drifting classifier
 * fails here first. Index kinds stay open strings — PostgreSQL access
 * methods are an open set (btree today, gin/brin tomorrow).
 */

export type SchemaExplorerDemoDto = SchemaExplorerDemoData;

const CONSTRAINT_KINDS: readonly string[] = ['PRIMARY KEY', 'FOREIGN KEY', 'UNIQUE', 'CHECK', 'OTHER'];

/** Deserialize + validate the GET /api/demo/db/schema-explorer payload. */
export function schemaExplorerDtoFromJson(body: unknown): SchemaExplorerDemoDto {
  if (!isRecord(body)) throw new Error('schema-explorer payload must be an object');
  return {
    tables: requiredArray(body, 'tables').map(tableFromJson),
    constraints: requiredArray(body, 'constraints').map(constraintFromJson),
    indexes: requiredArray(body, 'indexes').map(indexFromJson),
    history: requiredArray(body, 'history').map(historyFromJson),
    notes: requiredArray(body, 'notes').map(noteFromJson),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toSchemaExplorerDemo(dto: SchemaExplorerDemoDto): SchemaExplorerDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function tableFromJson(body: unknown): SchemaExplorerTable {
  if (!isRecord(body)) throw new Error('table must be an object');
  return {
    name: requiredString(body, 'name'),
    purpose: requiredText(body, 'purpose'),
    columns: requiredArray(body, 'columns').map(columnFromJson),
  };
}

function columnFromJson(body: unknown): SchemaExplorerColumn {
  if (!isRecord(body)) throw new Error('column must be an object');
  return {
    name: requiredString(body, 'name'),
    type: requiredString(body, 'type'),
    nullable: requiredBoolean(body, 'nullable'),
    default: requiredText(body, 'default'),
  };
}

function constraintFromJson(body: unknown): SchemaExplorerConstraint {
  if (!isRecord(body)) throw new Error('constraint must be an object');
  const kind = requiredString(body, 'kind');
  if (!CONSTRAINT_KINDS.includes(kind)) {
    throw new Error(`unknown constraint kind "${kind}"`);
  }
  return {
    table: requiredString(body, 'table'),
    name: requiredString(body, 'name'),
    kind,
    definition: requiredString(body, 'definition'),
  };
}

function indexFromJson(body: unknown): SchemaExplorerIndex {
  if (!isRecord(body)) throw new Error('index must be an object');
  return {
    table: requiredString(body, 'table'),
    name: requiredString(body, 'name'),
    kind: requiredString(body, 'kind'),
    columns: requiredArray(body, 'columns').map((column) => {
      if (typeof column !== 'string' || column === '') {
        throw new Error('expected a non-empty index column name');
      }
      return column;
    }),
    unique: requiredBoolean(body, 'unique'),
    primary: requiredBoolean(body, 'primary'),
    definition: requiredString(body, 'definition'),
  };
}

function historyFromJson(body: unknown): SchemaExplorerHistory {
  if (!isRecord(body)) throw new Error('migration history row must be an object');
  return {
    version: requiredNumber(body, 'version'),
    applied: requiredBoolean(body, 'applied'),
    // Varies per environment by design (when THIS database ran the
    // migration) — a string, asserted by shape only.
    appliedAt: requiredText(body, 'appliedAt'),
  };
}

function noteFromJson(body: unknown): SchemaExplorerNote {
  if (!isRecord(body)) throw new Error('rationale note must be an object');
  return {
    subject: requiredString(body, 'subject'),
    note: requiredString(body, 'note'),
  };
}
