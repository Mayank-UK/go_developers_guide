import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { ServerFailure } from '../../core/error-handling/failures';
import type { PostsRepository } from './post-repository';
import type { PostPage, PostsListQuery } from './post';
import { ListPostsUseCase } from './list-posts-usecase';

/**
 * ListPostsUseCase spec — a thin pass-through seam, so the contract to
 * pin is exactly that: the query flows through untouched and the
 * repository's Result (the page, or the 422 envelope a broken query
 * produces) arrives unchanged, never thrown.
 */

const page: PostPage = {
  items: [
    {
      id: 'p-7',
      title: 'A post from the spec',
      body: 'The stored body.',
      tags: ['go', 'api'],
      createdAt: '2026-02-01T10:00:00Z',
      updatedAt: '2026-02-01T10:00:00Z',
    },
  ],
  page: 1,
  limit: 5,
  totalItems: 7,
  totalPages: 2,
  hasNextPage: true,
  hasPrevPage: false,
};

function recordingRepository(result: Result<PostPage>): PostsRepository & { seen: PostsListQuery[] } {
  const seen: PostsListQuery[] = [];
  return {
    seen,
    list: (query) => {
      seen.push(query);
      return Promise.resolve(result);
    },
    create: () => Promise.reject(new Error('not under test')),
    replace: () => Promise.reject(new Error('not under test')),
    remove: () => Promise.reject(new Error('not under test')),
  };
}

describe('ListPostsUseCase', () => {
  it('passes the query through and returns the page unchanged', async () => {
    const repository = recordingRepository(success(page));

    const result = await new ListPostsUseCase(repository).execute({ page: 2, limit: 3, tag: 'go', sort: 'oldest' });

    expect(result).toEqual(success(page));
    expect(repository.seen).toEqual([{ page: 2, limit: 3, tag: 'go', sort: 'oldest' }]);
  });

  it('returns a Failed Result for a broken query (the 422 envelope), never throws', async () => {
    const repository = recordingRepository(
      failed(
        new ServerFailure('Invalid post list query.', {
          code: 'validation_failed',
          status: 422,
          requestId: 'req-9',
        }),
      ),
    );

    const result = await new ListPostsUseCase(repository).execute({ limit: 999 });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('validation_failed');
      expect(result.failure.status).toBe(422);
    }
  });
});
