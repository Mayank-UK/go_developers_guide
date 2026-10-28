import type {
  AssertionRow,
  BoundaryCheck,
  EnforcementRun,
  FieldRow,
  RuntimeTypesDemoData,
  TypeRow,
  ZeroValueRow,
} from '../../domain/demo/runtime-types';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * RuntimeTypesDTO — the wire/transport shape of the runtime-types demo
 * payload. Mirrors the domain model 1:1 (the interface is the seam where
 * wire and domain may diverge); the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException.
 *
 * Two fields are deliberately `requiredText` (present-but-empty passes)
 * rather than `requiredString`: `pkgPath` is `""` for every predeclared
 * type (string, int, slices of them), and `blockID` is `""` for the
 * comma-ok assertion that does not match — empty is DATA there, not
 * absence. `jsonTag` follows the same rule: reflect reports `""` for an
 * untagged field.
 */

export type RuntimeTypesDemoDto = RuntimeTypesDemoData;

/** Deserialize + validate the GET /api/demo/go/runtime-types payload. */
export function runtimeTypesDtoFromJson(body: unknown): RuntimeTypesDemoDto {
  if (!isRecord(body)) throw new Error('runtime-types payload must be an object');
  return {
    inspections: requiredArray(body, 'inspections').map(typeRowFromJson),
    fields: requiredArray(body, 'fields').map(fieldRowFromJson),
    assertions: requiredArray(body, 'assertions').map(assertionRowFromJson),
    zeroValues: requiredArray(body, 'zeroValues').map(zeroValueRowFromJson),
    enforcement: enforcementFromJson(body.enforcement),
    boundary: boundaryFromJson(body.boundary),
    // The tables' teaching sentences — prose the server teaches, the
    // client renders (never authors).
    fieldsNote: requiredString(body, 'fieldsNote'),
    assertionsNote: requiredString(body, 'assertionsNote'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toRuntimeTypesDemo(dto: RuntimeTypesDemoDto): RuntimeTypesDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function typeRowFromJson(body: unknown): TypeRow {
  if (!isRecord(body)) throw new Error('type row must be an object');
  return {
    expression: requiredString(body, 'expression'),
    type: requiredString(body, 'type'),
    kind: requiredString(body, 'kind'),
    pkgPath: requiredText(body, 'pkgPath'),
    sizeBytes: requiredNumber(body, 'sizeBytes'),
  };
}

function fieldRowFromJson(body: unknown): FieldRow {
  if (!isRecord(body)) throw new Error('field row must be an object');
  return {
    name: requiredString(body, 'name'),
    type: requiredString(body, 'type'),
    jsonTag: requiredText(body, 'jsonTag'),
  };
}

function assertionRowFromJson(body: unknown): AssertionRow {
  if (!isRecord(body)) throw new Error('assertion row must be an object');
  return {
    expression: requiredString(body, 'expression'),
    dynamicType: requiredString(body, 'dynamicType'),
    ok: requiredBoolean(body, 'ok'),
    blockID: requiredText(body, 'blockID'),
  };
}

function zeroValueRowFromJson(body: unknown): ZeroValueRow {
  if (!isRecord(body)) throw new Error('zero-value row must be an object');
  return {
    declaration: requiredString(body, 'declaration'),
    type: requiredString(body, 'type'),
    display: requiredString(body, 'display'),
    isZero: requiredBoolean(body, 'isZero'),
  };
}

function enforcementFromJson(body: unknown): EnforcementRun {
  if (!isRecord(body)) throw new Error('enforcement run must be an object');
  return {
    function: requiredString(body, 'function'),
    signature: requiredString(body, 'signature'),
    call: requiredString(body, 'call'),
    compilerVerdict: requiredString(body, 'compilerVerdict'),
    runtimeOutcome: requiredString(body, 'runtimeOutcome'),
  };
}

function boundaryFromJson(body: unknown): BoundaryCheck {
  if (!isRecord(body)) throw new Error('boundary check must be an object');
  return {
    decodeTarget: requiredString(body, 'decodeTarget'),
    badInput: requiredString(body, 'badInput'),
    outcome: requiredString(body, 'outcome'),
    errorType: requiredString(body, 'errorType'),
    message: requiredString(body, 'message'),
  };
}
