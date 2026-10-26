import type {
  MigrationsDemoData,
  MigrationsHistoryRow,
  TxStoryStep,
} from '../../domain/demo/db-migrations';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * MigrationsDTO — the wire/transport shape of the transactions demo
 * payload. `appliedAt` is requiredText (present-but-empty passes): the
 * stamp varies per environment and is displayed as data.
 */

export type MigrationsDemoDto = MigrationsDemoData;

/** Deserialize + validate the GET /api/demo/db/migrations payload. */
export function migrationsDtoFromJson(body: unknown): MigrationsDemoDto {
  if (!isRecord(body)) throw new Error('migrations payload must be an object');
  return {
    history: requiredArray(body, 'history').map(historyFromJson),
    steps: requiredArray(body, 'steps').map(stepFromJson),
    summary: requiredString(body, 'summary'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toMigrationsDemo(dto: MigrationsDemoDto): MigrationsDemoData {
  return dto;
}

function historyFromJson(body: unknown): MigrationsHistoryRow {
  if (!isRecord(body)) throw new Error('history row must be an object');
  return {
    version: requiredNumber(body, 'version'),
    applied: requiredBoolean(body, 'applied'),
    appliedAt: requiredText(body, 'appliedAt'),
  };
}

function stepFromJson(body: unknown): TxStoryStep {
  if (!isRecord(body)) throw new Error('tx story step must be an object');
  return {
    step: requiredNumber(body, 'step'),
    action: requiredString(body, 'action'),
    sql: requiredString(body, 'sql'),
    postsVisible: requiredNumber(body, 'postsVisible'),
    note: requiredString(body, 'note'),
  };
}
