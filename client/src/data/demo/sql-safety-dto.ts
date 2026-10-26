import type { SQLSafetyAttackRow, SQLSafetyDemoData } from '../../domain/demo/sql-safety';
import { isRecord, requiredArray, requiredNumber, requiredString } from '../content/validate';

/**
 * SQLSafetyDTO — the wire/transport shape of the sql-safety payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException.
 */

export type SQLSafetyDemoDto = SQLSafetyDemoData;

/** Deserialize + validate the GET /api/demo/db/sql-safety payload. */
export function sqlSafetyDtoFromJson(body: unknown): SQLSafetyDemoDto {
  if (!isRecord(body)) throw new Error('sql-safety payload must be an object');
  return {
    attacks: requiredArray(body, 'attacks').map(attackFromJson),
    summary: requiredString(body, 'summary'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toSQLSafetyDemo(dto: SQLSafetyDemoDto): SQLSafetyDemoData {
  return dto;
}

function attackFromJson(body: unknown): SQLSafetyAttackRow {
  if (!isRecord(body)) throw new Error('attack row must be an object');
  return {
    input: requiredString(body, 'input'),
    intent: requiredString(body, 'intent'),
    parameterizedSql: requiredString(body, 'parameterizedSql'),
    rowsMatched: requiredNumber(body, 'rowsMatched'),
    postsAfter: requiredNumber(body, 'postsAfter'),
    assembledSql: requiredString(body, 'assembledSql'),
    assembledEffect: requiredString(body, 'assembledEffect'),
    verdict: requiredString(body, 'verdict'),
  };
}
