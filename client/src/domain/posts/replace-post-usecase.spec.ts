import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { ServerFailure } from '../../core/error-handling/failures';
import type { PostsRepository } from './post-repository';
import type { Post, PostInput } from './post';
import { ReplacePostUseCase } from './replace-post-usecase';

/**
 * ReplacePostUseCase spec — thin seam: id + input flow to the repository
 * together (the PUT pair), and the replaced post (or the 404 envelope a
 * miss produces) arrives as the same Result, never thrown.
 */

const replaced: Post = {
  id: 'p-2',
  title: 'The whole next state',
  body: 'Full replacement.',
  tags: ['edited'],
  createdAt: '2026-01-07T14:40:00Z',
  updatedAt: '2026-02-02T09:00:00Z',
};

const input: PostInput = { title: 'The whole next state', body: 'Full replacement.', tags: ['edited'] };

function recordingRepository(result: Result<Post>): PostsRepository & { seen: { id: string; input: PostInput }[] } {
  const seen: { id: string; input: PostInput }[] = [];
  return {
    seen,
    list: () => Promise.reject(new Error('not under test')),
    create: () => Promise.reject(new Error('not under test')),
    replace: (id, value) => {
      seen.push({ id, input: value });
      return Promise.resolve(result);
    },
    remove: () => Promise.reject(new Error('not under test')),
  };
}

describe('ReplacePostUseCase', () => {
  it('carries the id with the input for a full-replace PUT', async () => {
    const repository = recordingRepository(success(replaced));

    const result = await new ReplacePostUseCase(repository).execute('p-2', input);

    expect(result).toEqual(success(replaced));
    expect(repository.seen).toEqual([{ id: 'p-2', input }]);
  });

  it('returns the 404 envelope as a Failed Result', async () => {
    const repository = recordingRepository(
      failed(
        new ServerFailure('Post "p-404" not found.', {
          code: 'not_found',
          status: 404,
          requestId: 'req-12',
        }),
      ),
    );

    const result = await new ReplacePostUseCase(repository).execute('p-404', input);

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('not_found');
      expect(result.failure.status).toBe(404);
    }
  });
});
