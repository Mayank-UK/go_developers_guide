import { describe, expect, it } from 'vitest';
import { migrationsDtoFromJson } from './db-migrations-dto';

describe('migrationsDtoFromJson', () => {
  const payload = {
    history: [{ version: 0, applied: true, appliedAt: '2026-10-26' }],
    steps: [
      { step: 1, action: 'begin', sql: 'BEGIN', postsVisible: 6, note: 'a unit opens' },
    ],
    summary: 'atomicity',
  };

  it('builds the DTO from a well-formed payload', () => {
    const dto = migrationsDtoFromJson(payload);
    expect(dto.history[0]?.version).toBe(0);
    expect(dto.steps[0]?.postsVisible).toBe(6);
  });

  it('refuses malformed payloads', () => {
    expect(() => migrationsDtoFromJson([])).toThrow();
    expect(() => migrationsDtoFromJson({ history: [], steps: [{}] })).toThrow();
  });
});
