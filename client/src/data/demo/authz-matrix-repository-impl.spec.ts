import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { AuthzMatrixApi } from './authz-matrix-api';
import { AuthzMatrixRepositoryImpl } from './authz-matrix-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * AuthzMatrixRepositoryImpl spec — fixture-driven (happy + failure): the
 * matrix payload (beats with cell citations, proofs, reasoning)
 * round-trips; a poisoned payload and a network failure resolve as Failed
 * Results, never throws.
 */

const demoPayload = {
  mechanism: 'rbac-matrix',
  beats: [
    {
      beat: 'the roster',
      detail: 'RBAC starts as a table: role → permission set, defined in ONE registry.',
      verdict: 'info',
      facts: [
        { label: 'reader', value: 'posts:read' },
        { label: 'author', value: 'posts:read posts:write' },
      ],
    },
    {
      beat: 'owner, editor, stranger',
      detail:
        'ada posts:write p-sessions: allow — object-level pass · grace posts:write p-sessions: allow — editor writes span every post.',
      verdict: 'ok',
      facts: [],
    },
    {
      beat: 'deny by default',
      detail: 'guest posts:write p-sessions: deny — no role in [] grants "posts:write".',
      verdict: 'rejected',
      facts: [],
    },
  ],
  proofs: [
    { label: 'matrix size', value: '45 evaluated cells (7 subjects × actions × objects)' },
    { label: 'write column', value: '9 of 21 write cells allow — the rest deny at scope, role, or object gates' },
  ],
  reasoning: 'Each gate only ever narrows the one before it.',
};

function makeRepository(): AuthzMatrixRepositoryImpl {
  return new AuthzMatrixRepositoryImpl(new AuthzMatrixApi(createAppConfig()));
}

describe('AuthzMatrixRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/authz/matrix', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.mechanism).toBe('rbac-matrix');
      expect(result.value.beats).toHaveLength(3);
      expect(result.value.beats[0]?.facts[0]?.value).toBe('posts:read');
      expect(result.value.beats[2]?.verdict).toBe('rejected');
      expect(result.value.proofs[1]?.label).toBe('write column');
    }
  });

  it('rejects a payload whose beat lacks its citation detail', async () => {
    const poisoned = {
      ...demoPayload,
      beats: [{ ...demoPayload.beats[0], detail: '' }],
    };
    stubApiFetch([{ url: '/api/demo/authz/matrix', body: poisoned }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
