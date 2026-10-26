import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { PostsWorkbenchApi } from './posts-workbench-api';
import { PostsWorkbenchRepositoryImpl } from './posts-workbench-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * PostsWorkbenchRepositoryImpl spec — fixture-driven (happy + failure):
 * the contract card round-trips into the domain shape; network / envelope
 * / malformed-payload failures all resolve as Failed Results, never
 * throws.
 */

const cardPayload = {
  resource: {
    name: 'posts',
    collectionPath: '/api/posts',
    itemPath: '/api/posts/{id}',
    wireShape: '{"id","title","body","tags","createdAt","updatedAt"}',
  },
  routes: [
    {
      id: 'list',
      method: 'GET',
      path: '/api/posts',
      purpose: 'one page of the collection',
      success: { status: 200, note: 'page envelope' },
      errors: [{ status: 422, code: 'validation_failed', when: 'broken query' }],
    },
  ],
  pagination: {
    defaultLimit: 10,
    maxLimit: 50,
    defaultPage: 1,
    defaultSort: 'newest',
    sortOptions: ['newest', 'oldest', 'title'],
    envelopeFields: ['items', 'page', 'limit'],
    emptyPageRule: 'a past-the-end page is valid and empty',
  },
  filtering: { param: 'tag', semantics: 'exact match', example: '/api/posts?tag=go' },
  versioning: { prefix: '/api', strategy: 'unversioned', contentExample: 'version field', whenToVersion: 'on breaking change' },
  deleteSemantics: { onMiss: 404, chosen: 'strict', rationale: 'truth', alternative: 'idempotent 204' },
  envelope: {
    wireShape: '{"error":{…}}',
    members: [{ name: 'code', meaning: 'taxonomy bucket' }],
    rfc9457Verdict: 'matches in spirit',
  },
};

function makeRepository(): PostsWorkbenchRepositoryImpl {
  return new PostsWorkbenchRepositoryImpl(new PostsWorkbenchApi(createAppConfig()));
}

describe('PostsWorkbenchRepositoryImpl', () => {
  it('returns a Success carrying the validated contract card', async () => {
    stubApiFetch([{ url: '/api/demo/api/posts-workbench', body: cardPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.resource.collectionPath).toBe('/api/posts');
      expect(result.value.routes[0]?.success.status).toBe(200);
      expect(result.value.deleteSemantics.onMiss).toBe(404);
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

  it('guards an error envelope into a Failed Result with its diagnostics', async () => {
    stubApiFetch([
      {
        url: '/api/demo/api/posts-workbench',
        status: 503,
        body: { error: { code: 'service_unavailable', message: 'calm down', requestId: 'req-9' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('calm down', {
          code: 'service_unavailable',
          status: 503,
          requestId: 'req-9',
        }),
      ),
    );
  });

  it('guards a malformed card into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // Missing the routes table — the DTO validator must reject it.
      { url: '/api/demo/api/posts-workbench', body: { resource: cardPayload.resource } },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('The server response did not match the expected shape.', {
          code: 'invalid_response',
          status: 200,
        }),
      ),
    );
  });
});
