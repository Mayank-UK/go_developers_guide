import type {
  BestChecklistAreaRow,
  BestChecklistDemoData,
  BestChecklistItemRow,
  BestChecklistSummary,
  BestNotDoneRow,
} from '../../domain/demo/best-checklist';
import {
  isRecord,
  optionalString,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * BestChecklistDTO — the wire/transport shape of the self-audit
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY
 * the listed fields (whitelist) and throws on any shape violation,
 * which parseDto upgrades to an InvalidResponseException.
 *
 * Present-but-empty is real data on this payload: the honest computing
 * placeholder the server serves before its boot audit finishes carries
 * empty strings and empty arrays, so string fields use requiredText
 * (never requiredString) and collections stay requiredArray.
 */

export type BestChecklistDemoDto = BestChecklistDemoData;

const CHECK_KINDS = ['live', 'exec', 'file', 'manual'] as const;
const VERDICTS = ['pass', 'fail', 'manual'] as const;

function itemFromJson(value: unknown): BestChecklistItemRow {
  if (!isRecord(value)) throw new Error('checklist row must be an object');
  const checkKind = requiredString(value, 'checkKind');
  const verdict = requiredString(value, 'verdict');
  if (!CHECK_KINDS.includes(checkKind as (typeof CHECK_KINDS)[number])) {
    throw new Error(`unknown checkKind "${checkKind}"`);
  }
  if (!VERDICTS.includes(verdict as (typeof VERDICTS)[number])) {
    throw new Error(`unknown verdict "${verdict}"`);
  }
  return {
    id: requiredString(value, 'id'),
    practice: requiredString(value, 'practice'),
    checkKind: checkKind as BestChecklistItemRow['checkKind'],
    verdict: verdict as BestChecklistItemRow['verdict'],
    evidence: requiredArray(value, 'evidence').map(String),
    whyManual: optionalString(value, 'whyManual'),
  };
}

function areaFromJson(value: unknown): BestChecklistAreaRow {
  if (!isRecord(value)) throw new Error('checklist area must be an object');
  return {
    id: requiredString(value, 'id'),
    title: requiredString(value, 'title'),
    note: requiredText(value, 'note'),
    items: requiredArray(value, 'items').map(itemFromJson),
  };
}

function notDoneFromJson(value: unknown): BestNotDoneRow {
  if (!isRecord(value)) throw new Error('not-done row must be an object');
  return {
    id: requiredString(value, 'id'),
    notDone: requiredString(value, 'notDone'),
    reason: requiredString(value, 'reason'),
    artifacts: requiredArray(value, 'artifacts').map(String),
  };
}

function summaryFromJson(value: unknown): BestChecklistSummary {
  if (!isRecord(value)) throw new Error('summary must be an object');
  return {
    items: requiredNumber(value, 'items'),
    passed: requiredNumber(value, 'passed'),
    failed: requiredNumber(value, 'failed'),
    manual: requiredNumber(value, 'manual'),
  };
}

/** Deserialize + validate the GET /api/demo/best/checklist payload. */
export function bestChecklistDtoFromJson(body: unknown): BestChecklistDemoDto {
  if (!isRecord(body)) throw new Error('best-checklist payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    computedAt: requiredText(body, 'computedAt'),
    areas: requiredArray(body, 'areas').map(areaFromJson),
    notDones: requiredArray(body, 'notDones').map(notDoneFromJson),
    summary: summaryFromJson(requiredRecord(body, 'summary')),
    honestyNote: requiredText(body, 'honestyNote'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toBestChecklistDemo(dto: BestChecklistDemoDto): BestChecklistDemoData {
  return dto;
}
