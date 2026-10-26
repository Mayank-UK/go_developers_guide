import { describe, expect, it } from 'vitest';
import { auditDtoFromJson } from './db-audit-dto';

describe('auditDtoFromJson', () => {
  const payload = {
    steps: [
      { phase: 'commit-done', action: 'COMMIT', liveTitle: 'audit scratch (edited)', auditRows: 2, note: 'both true for everyone' },
    ],
    committedTogether: true,
    rolledBackTogether: true,
    summary: 'same transaction, both writes',
  };

  it('builds the DTO from a well-formed payload', () => {
    const dto = auditDtoFromJson(payload);
    expect(dto.steps[0]?.auditRows).toBe(2);
    expect(dto.rolledBackTogether).toBe(true);
  });

  it('refuses malformed payloads', () => {
    expect(() => auditDtoFromJson([])).toThrow();
    expect(() => auditDtoFromJson({ ...payload, committedTogether: 'yes' })).toThrow();
  });
});
