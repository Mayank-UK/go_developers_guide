import { describe, expect, it } from 'vitest';
import { authzMatrixDtoFromJson, toAuthzMatrixDemo } from './authz-matrix-dto';
import type { AuthzMatrixDemoData } from '../../domain/demo/authz-matrix';

/**
 * AuthzMatrixDTO spec — whitelist parsing with shape violations throwing
 * (parseDto upgrades them to InvalidResponseException at the boundary).
 * The verdict vocabulary is interpreted by coloring, never parsed, so
 * verdicts validate as non-empty strings only.
 */

const wellFormed: AuthzMatrixDemoData = {
  mechanism: 'rbac-matrix',
  beats: [
    {
      beat: 'the roster',
      detail: 'RBAC starts as a table: role → permission set.',
      verdict: 'info',
      facts: [
        { label: 'reader', value: 'posts:read' },
        { label: 'admin', value: 'posts:read posts:write users:manage' },
      ],
    },
    {
      beat: 'owner, editor, stranger',
      detail:
        'ada posts:write p-sessions: allow — object-level pass · ada posts:write p-draft: deny — object-level refusal.',
      verdict: 'ok',
      facts: [],
    },
    {
      beat: 'deny by default',
      detail: 'guest posts:write p-sessions: deny — no role grants.',
      verdict: 'rejected',
      facts: [],
    },
  ],
  proofs: [
    { label: 'matrix size', value: '45 evaluated cells (7 subjects × actions × objects)' },
    { label: 'deny by default', value: 'guest (no role) denied every action: true' },
  ],
  reasoning: 'Each gate only ever narrows the one before it.',
};

describe('authzMatrixDtoFromJson', () => {
  it('parses the well-formed payload field by field', () => {
    const dto = authzMatrixDtoFromJson(wellFormed);

    expect(dto.mechanism).toBe('rbac-matrix');
    expect(dto.beats).toHaveLength(3);
    expect(dto.beats[0]?.facts[1]?.value).toBe('posts:read posts:write users:manage');
    expect(dto.beats[2]?.verdict).toBe('rejected');
    expect(dto.proofs[0]?.label).toBe('matrix size');
    expect(dto.reasoning).toContain('narrows');
  });

  it('is the identity into the domain model while 1:1', () => {
    expect(toAuthzMatrixDemo(wellFormed)).toEqual(wellFormed);
  });

  it('throws when the payload is not an object', () => {
    expect(() => authzMatrixDtoFromJson('rbac-matrix')).toThrow();
    expect(() => authzMatrixDtoFromJson(null)).toThrow();
  });

  it('throws when a required top-level field is missing or empty', () => {
    for (const key of ['mechanism', 'beats', 'proofs', 'reasoning'] as const) {
      const poisoned: Record<string, unknown> = { ...wellFormed };
      delete poisoned[key];
      expect(() => authzMatrixDtoFromJson(poisoned), key).toThrow();
    }
    expect(() => authzMatrixDtoFromJson({ ...wellFormed, mechanism: '' })).toThrow();
  });

  it('throws when a beat or fact has the wrong shape', () => {
    expect(() =>
      authzMatrixDtoFromJson({ ...wellFormed, beats: [{ ...wellFormed.beats[0], verdict: '' }] }),
    ).toThrow();
    expect(() =>
      authzMatrixDtoFromJson({ ...wellFormed, beats: [{ ...wellFormed.beats[0], facts: 'none' }] }),
    ).toThrow();
    expect(() =>
      authzMatrixDtoFromJson({
        ...wellFormed,
        proofs: [{ label: 'matrix size', value: 45 }],
      }),
    ).toThrow();
  });
});
