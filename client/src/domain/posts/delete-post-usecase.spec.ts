import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure, ServerFailure } from '../../core/error-handling/failures';
import type { PostsRepository } from './post-repository';
import { DeletePostUseCase } from './delete-post-usecase';

/**
 * DeletePostUseCase spec — thin seam: the id flows down, the 204 success
 * resolves as Success(undefined), and both failure modes a real delete
 * meets (the strict-stance 404 envelope, a network drop) arrive as Failed
 * Results, never thrown.
 */

function recordingRepository(result: Result<void>): PostsRepository & { seen: string[] } {
  const seen: string[] = [];
  return {
    seen,
    list: () => Promise.reject(new Error('not under test')),
    create: () => Promise.reject(new Error('not under test')),
    replace: () => Promise.reject(new Error('not under test')),
    remove: (id) => {
      seen.push(id);
      return Promise.resolve(result);
    },
  };
}

describe('DeletePostUseCase', () => {
  it('resolves success (the 204 path) with the id handed down', async () => {
    const repository = recordingRepository(success(undefined));

    const result = await new DeletePostUseCase(repository).execute('p-3');

    expect(result).toEqual(success(undefined));
    expect(repository.seen).toEqual(['p-3']);
  });

  it('returns the strict-stance 404 as a Failed Result', async () => {
    const repository = recordingRepository(
      failed(
        new ServerFailure('Post "p-3" not found.', {
          code: 'not_found',
          status: 404,
          requestId: 'req-11',
        }),
      ),
    );

    const result = await new DeletePostUseCase(repository).execute('p-3');

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('not_found');
      expect(result.failure.status).toBe(404);
    }
  });

  it('carries network failures as Failed Results too', async () => {
    const repository = recordingRepository(failed(new NetworkFailure('Could not reach the server.')));

    const result = await new DeletePostUseCase(repository).execute('p-3');

    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.failure.status).toBe(0);
  });
});
