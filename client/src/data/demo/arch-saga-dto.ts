import type {
  ArchSagaCountRow,
  ArchSagaDemoData,
  ArchSagaLedgerRow,
  ArchSagaRunPanel,
  ArchSagaStepRow,
  ArchSagaSurfaceRow,
} from '../../domain/demo/arch-saga';
import {
  isRecord,
  optionalArray,
  optionalString,
  requiredArray,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ArchSagaDTO — the wire/transport shape of the saga payload. Mirrors
 * the domain model 1:1; the `fromJson` builds ONLY the listed fields
 * (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException.
 *
 * Optional wire fields mirror the server's omitempty exactly: a
 * SUCCESS or CANCELLED run carries no `trigger` (nothing failed) and no
 * `replay` (only the failure run re-compensates), so those decode as
 * undefined. A step's `undo` and a row's `detail` are required but MAY
 * be empty — empty is real data there (a non-compensatable step, a
 * success with no error text) — which is why they use requiredText,
 * never requiredString.
 */

export type ArchSagaDemoDto = ArchSagaDemoData;

function stepRowFromJson(value: unknown): ArchSagaStepRow {
  if (!isRecord(value)) throw new Error('step row must be an object');
  return {
    order: requiredNumber(value, 'order'),
    name: requiredString(value, 'name'),
    forward: requiredText(value, 'forward'),
    undo: requiredText(value, 'undo'),
  };
}

function ledgerRowFromJson(value: unknown): ArchSagaLedgerRow {
  if (!isRecord(value)) throw new Error('ledger row must be an object');
  return {
    seq: requiredNumber(value, 'seq'),
    step: requiredString(value, 'step'),
    phase: requiredString(value, 'phase'),
    outcome: requiredString(value, 'outcome'),
    detail: requiredText(value, 'detail'),
  };
}

function countRowFromJson(value: unknown): ArchSagaCountRow {
  if (!isRecord(value)) throw new Error('count row must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredNumber(value, 'value'),
  };
}

function runPanelFromJson(value: unknown): ArchSagaRunPanel {
  if (!isRecord(value)) throw new Error('run panel must be an object');
  return {
    id: requiredString(value, 'id'),
    title: requiredString(value, 'title'),
    lead: requiredText(value, 'lead'),
    script: requiredText(value, 'script'),
    status: requiredString(value, 'status'),
    trigger: optionalString(value, 'trigger'),
    ledger: requiredArray(value, 'ledger').map(ledgerRowFromJson),
    worldLog: requiredArray(value, 'worldLog').map(String),
    counts: requiredArray(value, 'counts').map(countRowFromJson),
    replay: optionalArray(value, 'replay')?.map(ledgerRowFromJson),
    note: requiredText(value, 'note'),
  };
}

function surfaceRowFromJson(value: unknown): ArchSagaSurfaceRow {
  if (!isRecord(value)) throw new Error('surface row must be an object');
  return {
    id: requiredString(value, 'id'),
    kind: requiredString(value, 'kind'),
    role: requiredText(value, 'role'),
  };
}

/** Deserialize + validate the GET /api/demo/arch/saga payload. */
export function archSagaDtoFromJson(body: unknown): ArchSagaDemoDto {
  if (!isRecord(body)) throw new Error('arch-saga payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    source: requiredString(body, 'source'),
    honestyNote: requiredText(body, 'honestyNote'),
    steps: requiredArray(body, 'steps').map(stepRowFromJson),
    runs: requiredArray(body, 'runs').map(runPanelFromJson),
    semantics: requiredArray(body, 'semantics').map(String),
    surface: requiredArray(body, 'surface').map(surfaceRowFromJson),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toArchSagaDemo(dto: ArchSagaDemoDto): ArchSagaDemoData {
  return dto;
}
