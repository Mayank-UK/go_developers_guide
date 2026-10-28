import type {
  ArchEventstoreCount,
  ArchEventstoreDemoData,
  ArchEventstoreInventoryState,
  ArchEventstorePanel,
  ArchEventstoreReplayEvidence,
  ArchEventstoreRow,
} from '../../domain/demo/arch-eventstore';
import {
  isRecord,
  optionalNumber,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ArchEventstoreDTO — the wire/transport shape of the event-store
 * exhibit payload. Mirrors the domain model 1:1; the `fromJson` builds
 * ONLY the listed fields (whitelist) and throws on any shape violation,
 * which parseDto upgrades to an InvalidResponseException.
 *
 * Optional wire fields mirror the server's omitempty exactly: only the
 * CONFLICT row carries expectedVersion / actualVersion (there was no
 * check to fail on any other row), so those decode as undefined on
 * every other row — present-but-empty is real data on the strings that
 * can be empty, which therefore use requiredText, never requiredString.
 */

export type ArchEventstoreDemoDto = ArchEventstoreDemoData;

function rowFromJson(value: unknown): ArchEventstoreRow {
  if (!isRecord(value)) throw new Error('exhibit row must be an object');
  return {
    step: requiredNumber(value, 'step'),
    call: requiredString(value, 'call'),
    input: requiredText(value, 'input'),
    outcome: requiredString(value, 'outcome'),
    version: requiredNumber(value, 'version'),
    expectedVersion: optionalNumber(value, 'expectedVersion'),
    actualVersion: optionalNumber(value, 'actualVersion'),
    detail: requiredText(value, 'detail'),
  };
}

function countFromJson(value: unknown): ArchEventstoreCount {
  if (!isRecord(value)) throw new Error('count row must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredNumber(value, 'value'),
  };
}

/** reservationsFromJson validates the per-ref reservation map: every value must be a number. */
function reservationsFromJson(source: Record<string, unknown>, key: string): Record<string, number> {
  const record = requiredRecord(source, key);
  const reservations: Record<string, number> = {};
  for (const [ref, qty] of Object.entries(record)) {
    if (typeof qty !== 'number' || !Number.isFinite(qty)) {
      throw new Error(`reservations["${ref}"] must be a finite number, got ${String(qty)}`);
    }
    reservations[ref] = qty;
  }
  return reservations;
}

function inventoryStateFromJson(value: unknown): ArchEventstoreInventoryState {
  if (!isRecord(value)) throw new Error('inventory state must be an object');
  return {
    onHand: requiredNumber(value, 'onHand'),
    reserved: requiredNumber(value, 'reserved'),
    available: requiredNumber(value, 'available'),
    reservations: reservationsFromJson(value, 'reservations'),
  };
}

function panelFromJson(value: unknown): ArchEventstorePanel {
  if (!isRecord(value)) throw new Error('exhibit panel must be an object');
  return {
    id: requiredString(value, 'id'),
    title: requiredString(value, 'title'),
    lead: requiredText(value, 'lead'),
    config: requiredArray(value, 'config').map(String),
    rows: requiredArray(value, 'rows').map(rowFromJson),
    counts: requiredArray(value, 'counts').map(countFromJson),
  };
}

function replayEvidenceFromJson(value: unknown): ArchEventstoreReplayEvidence {
  if (!isRecord(value)) throw new Error('replay evidence must be an object');
  return {
    foldedEvents: requiredNumber(value, 'foldedEvents'),
    appliedInOrder: requiredArray(value, 'appliedInOrder').map(String),
    state: inventoryStateFromJson(requiredRecord(value, 'state')),
    naiveExpected: inventoryStateFromJson(requiredRecord(value, 'naiveExpected')),
    rebuildsRun: requiredNumber(value, 'rebuildsRun'),
    byteIdentical: requiredBoolean(value, 'byteIdentical'),
    absentStreamId: requiredString(value, 'absentStreamId'),
    absentState: inventoryStateFromJson(requiredRecord(value, 'absentState')),
    absentVersion: requiredNumber(value, 'absentVersion'),
    note: requiredText(value, 'note'),
  };
}

/** Deserialize + validate the GET /api/demo/arch/eventstore payload. */
export function archEventstoreDtoFromJson(body: unknown): ArchEventstoreDemoDto {
  if (!isRecord(body)) throw new Error('arch-eventstore payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    honestyNote: requiredText(body, 'honestyNote'),
    streamId: requiredString(body, 'streamId'),
    store: requiredArray(body, 'store').map(String),
    panels: requiredArray(body, 'panels').map(panelFromJson),
    replay: replayEvidenceFromJson(requiredRecord(body, 'replay')),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toArchEventstoreDemo(dto: ArchEventstoreDemoDto): ArchEventstoreDemoData {
  return dto;
}
