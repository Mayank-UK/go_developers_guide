import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { PostsWorkbenchDemoData } from './posts-workbench';
import type { PostsWorkbenchRepository } from './posts-workbench-repository';
import { GetPostsWorkbenchDemoUseCase } from './get-posts-workbench-demo-usecase';

/**
 * GetPostsWorkbenchDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

function fakeRepository(get: Result<PostsWorkbenchDemoData>): PostsWorkbenchRepository {
  return { get: () => Promise.resolve(get) };
}

const card: PostsWorkbenchDemoData = {
  resource: {
    name: 'posts',
    collectionPath: '/api/posts',
    itemPath: '/api/posts/{id}',
    wireShape: '{"id"}',
  },
  routes: [],
  pagination: {
    defaultLimit: 10,
    maxLimit: 50,
    defaultPage: 1,
    defaultSort: 'newest',
    sortOptions: ['newest'],
    envelopeFields: ['items'],
    emptyPageRule: 'valid and empty',
  },
  filtering: { param: 'tag', semantics: 'exact', example: '/api/posts?tag=go' },
  versioning: { prefix: '/api', strategy: 'unversioned', contentExample: 'version 1', whenToVersion: 'on break' },
  deleteSemantics: { onMiss: 404, chosen: 'strict', rationale: 'truth', alternative: '204' },
  envelope: { wireShape: '{"error":{}}', members: [], rfc9457Verdict: 'kindred' },
};

describe('GetPostsWorkbenchDemoUseCase', () => {
  it('returns the card the repository resolved, unchanged', async () => {
    const useCase = new GetPostsWorkbenchDemoUseCase(fakeRepository(success(card)));

    const result = await useCase.execute();

    expect(result).toEqual(success(card));
  });

  it('returns a Failed Result when the card cannot be loaded (never throws)', async () => {
    const useCase = new GetPostsWorkbenchDemoUseCase(
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
