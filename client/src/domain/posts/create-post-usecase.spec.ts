import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { ServerFailure } from '../../core/error-handling/failures';
import type { PostsRepository } from './post-repository';
import type { Post, PostInput } from './post';
import { CreatePostUseCase } from './create-post-usecase';

/**
 * CreatePostUseCase spec — thin seam: the input flows to the repository
 * untouched, and the stored post (or the 422 issue envelope a broken
 * draft produces) arrives as the same Result, never thrown.
 */

const stored: Post = {
  id: 'p-7',
  title: 'A post from the spec',
  body: 'The stored body.',
  tags: ['go', 'api'],
  createdAt: '2026-02-01T10:00:00Z',
  updatedAt: '2026-02-01T10:00:00Z',
};

const input: PostInput = { title: '  A post from the spec ', body: 'The stored body.', tags: ['go', 'api'] };

function recordingRepository(result: Result<Post>): PostsRepository & { seen: PostInput[] } {
  const seen: PostInput[] = [];
  return {
    seen,
    list: () => Promise.reject(new Error('not under test')),
    create: (value) => {
      seen.push(value);
      return Promise.resolve(result);
    },
    replace: () => Promise.reject(new Error('not under test')),
    remove: () => Promise.reject(new Error('not under test')),
  };
}

describe('CreatePostUseCase', () => {
  it('hands the input to the repository and returns the stored post', async () => {
    const repository = recordingRepository(success(stored));

    const result = await new CreatePostUseCase(repository).execute(input);

    expect(result).toEqual(success(stored));
    expect(repository.seen).toEqual([input]);
  });

  it('returns a Failed Result carrying the 422 issue envelope verbatim', async () => {
    const repository = recordingRepository(
      failed(
        new ServerFailure('Invalid post body.', {
          code: 'validation_failed',
          status: 422,
          requestId: 'req-10',
        }),
      ),
    );

    const result = await new CreatePostUseCase(repository).execute({ title: '', body: '', tags: [] });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.message).toBe('Invalid post body.');
      expect(result.failure.requestId).toBe('req-10');
    }
  });
});
