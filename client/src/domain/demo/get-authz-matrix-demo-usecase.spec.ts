import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { AuthzMatrixDemoData } from './authz-matrix';
import type { AuthzMatrixRepository } from './authz-matrix-repository';
import { GetAuthzMatrixDemoUseCase } from './get-authz-matrix-demo-usecase';

/**
 * GetAuthzMatrixDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

function fakeRepository(get: Result<AuthzMatrixDemoData>): AuthzMatrixRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: AuthzMatrixDemoData = {
  mechanism: 'rbac-matrix',
  beats: [
    {
      beat: 'the roster',
      detail: 'RBAC starts as a table.',
      verdict: 'info',
      facts: [{ label: 'reader', value: 'posts:read' }],
    },
  ],
  proofs: [{ label: 'matrix size', value: '45 evaluated cells' }],
  reasoning: 'Each gate only ever narrows the one before it.',
};

describe('GetAuthzMatrixDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetAuthzMatrixDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetAuthzMatrixDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
