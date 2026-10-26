import type { AuditDemoData, AuditStep } from '../../domain/demo/db-audit';
import { isRecord, requiredArray, requiredBoolean, requiredNumber, requiredString, requiredText } from '../content/validate';

/**
 * AuditDTO — the wire/transport shape of the audit demo payload.
 * `liveTitle` is requiredText: the pre-creation step's "(no post yet)"
 * placeholder is data, and a blank title would still render as data.
 */

export type AuditDemoDto = AuditDemoData;

/** Deserialize + validate the GET /api/demo/db/audit payload. */
export function auditDtoFromJson(body: unknown): AuditDemoDto {
  if (!isRecord(body)) throw new Error('audit payload must be an object');
  return {
    steps: requiredArray(body, 'steps').map(stepFromJson),
    committedTogether: requiredBoolean(body, 'committedTogether'),
    rolledBackTogether: requiredBoolean(body, 'rolledBackTogether'),
    summary: requiredString(body, 'summary'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toAuditDemo(dto: AuditDemoDto): AuditDemoData {
  return dto;
}

function stepFromJson(body: unknown): AuditStep {
  if (!isRecord(body)) throw new Error('audit step must be an object');
  return {
    phase: requiredString(body, 'phase'),
    action: requiredString(body, 'action'),
    liveTitle: requiredText(body, 'liveTitle'),
    auditRows: requiredNumber(body, 'auditRows'),
    note: requiredString(body, 'note'),
  };
}
