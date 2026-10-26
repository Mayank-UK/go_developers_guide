import { describe, expect, it } from 'vitest';
import { raceDtoFromJson } from './db-race-dto';

describe('raceDtoFromJson', () => {
  const payload = {
    isolation: 'read committed',
    scratchBefore: 'before',
    scratchAfter: 'after',
    steps: [
      { phase: 'reader-looks', actor: 'T2', action: 'SELECT', observed: 'old row', note: 'snapshot per statement' },
    ],
    lockWaited: false,
    serializableNote: 'Under SERIALIZABLE …',
  };

  it('builds the DTO from a well-formed payload', () => {
    const dto = raceDtoFromJson(payload);
    expect(dto.steps[0]?.phase).toBe('reader-looks');
    expect(dto.lockWaited).toBe(false);
  });

  it('refuses malformed payloads', () => {
    expect(() => raceDtoFromJson('x')).toThrow();
    expect(() => raceDtoFromJson({ ...payload, steps: [{}] })).toThrow();
  });
});
