import type {
  CompileProof,
  EmbeddingRow,
  InterfaceMatrixDemoData,
  InterfaceSpec,
  MatrixCell,
  SatisfactionRow,
  SwitchCaseRow,
} from '../../domain/demo/interface-matrix';
import {
  isRecord,
  optionalString,
  requiredArray,
  requiredBoolean,
  requiredString,
} from '../content/validate';

/**
 * InterfaceMatrixDTO — the wire/transport shape of the satisfaction-matrix
 * demo payload. Mirrors the domain model 1:1 (the interface is the seam
 * where wire and domain may diverge); the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException.
 *
 * Two honest empties: a row's `methods` may be `[]` (reflect lists only
 * exported methods — the sealed `content.Block` rows are the point), and a
 * type-switch row's `note` is optional (the Go field is omitempty; only
 * the rows carrying the case-order lesson include it).
 */

export type InterfaceMatrixDemoDto = InterfaceMatrixDemoData;

/** Deserialize + validate the GET /api/demo/iface/matrix payload. */
export function interfaceMatrixDtoFromJson(body: unknown): InterfaceMatrixDemoDto {
  if (!isRecord(body)) throw new Error('interface-matrix payload must be an object');
  return {
    interfaces: requiredArray(body, 'interfaces').map(interfaceSpecFromJson),
    rows: requiredArray(body, 'rows').map(satisfactionRowFromJson),
    embeddings: requiredArray(body, 'embeddings').map(embeddingRowFromJson),
    typeSwitch: requiredArray(body, 'typeSwitch').map(switchCaseRowFromJson),
    compileProofs: requiredArray(body, 'compileProofs').map(compileProofFromJson),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toInterfaceMatrixDemo(dto: InterfaceMatrixDemoDto): InterfaceMatrixDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function interfaceSpecFromJson(body: unknown): InterfaceSpec {
  if (!isRecord(body)) throw new Error('interface spec must be an object');
  return {
    name: requiredString(body, 'name'),
    method: requiredString(body, 'method'),
    pkg: requiredString(body, 'pkg'),
    note: requiredString(body, 'note'),
  };
}

function satisfactionRowFromJson(body: unknown): SatisfactionRow {
  if (!isRecord(body)) throw new Error('satisfaction row must be an object');
  return {
    expression: requiredString(body, 'expression'),
    type: requiredString(body, 'type'),
    kind: requiredString(body, 'kind'),
    methods: requiredArray(body, 'methods').map(requiredStringInArray),
    cells: requiredArray(body, 'cells').map(matrixCellFromJson),
  };
}

function matrixCellFromJson(body: unknown): MatrixCell {
  if (!isRecord(body)) throw new Error('matrix cell must be an object');
  return {
    iface: requiredString(body, 'iface'),
    satisfied: requiredBoolean(body, 'satisfied'),
  };
}

function embeddingRowFromJson(body: unknown): EmbeddingRow {
  if (!isRecord(body)) throw new Error('embedding row must be an object');
  return {
    expression: requiredString(body, 'expression'),
    embeds: requiredString(body, 'embeds'),
    type: requiredString(body, 'type'),
    iface: requiredString(body, 'iface'),
    satisfied: requiredBoolean(body, 'satisfied'),
    note: requiredString(body, 'note'),
  };
}

function switchCaseRowFromJson(body: unknown): SwitchCaseRow {
  if (!isRecord(body)) throw new Error('switch case row must be an object');
  return {
    input: requiredString(body, 'input'),
    matchedCase: requiredString(body, 'matchedCase'),
    result: requiredString(body, 'result'),
    note: optionalString(body, 'note'),
  };
}

function compileProofFromJson(body: unknown): CompileProof {
  if (!isRecord(body)) throw new Error('compile proof must be an object');
  return {
    line: requiredString(body, 'line'),
    pins: requiredString(body, 'pins'),
  };
}

function requiredStringInArray(value: unknown): string {
  if (typeof value !== 'string' || value === '') {
    throw new Error('expected a non-empty string inside the methods list');
  }
  return value;
}
