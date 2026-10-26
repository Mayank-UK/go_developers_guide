import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { PostsApi } from './posts-api';
import { PostsRepositoryImpl } from './posts-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * PostsRepositoryImpl spec — the REAL CRUD client's round-trip discipline:
 * every verb's happy path resolves Success with the validated domain
 * shape, and every failure mode the workbench must render honestly (the
 * 422 issue envelope, the strict-stance 404, network, malformed payload)
 * resolves as a Failed Result, never throws. The list round-trip pins the
 * URL contract too: sent controls ride the query string, omitted ones
 * stay omitted.
 */

const wirePost = {
  id: 'p-7',
  title: 'A post from the spec',
  body: 'The stored body.',
  tags: ['go', 'api'],
  createdAt: '2026-02-01T10:00:00Z',
  updatedAt: '2026-02-01T10:00:00Z',
};

const wirePage = {
  items: [wirePost],
  page: 2,
  limit: 3,
  totalItems: 7,
  totalPages: 3,
  hasNextPage: true,
  hasPrevPage: true,
};

function makeRepository(): PostsRepositoryImpl {
  return new PostsRepositoryImpl(new PostsApi(createAppConfig()));
}

describe('PostsRepositoryImpl', () => {
  it('lists with the query controls on the URL and resolves the page', async () => {
    let seenUrl = '';
    let seenInit: RequestInit | undefined;
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        seenUrl = typeof input === 'string' ? input : input.toString();
        seenInit = init;
        return {
          ok: true,
          status: 200,
          headers: {},
          json: async () => wirePage,
          text: async () => JSON.stringify(wirePage),
        };
      }),
    );

    const result = await makeRepository().list({ page: 2, limit: 3, tag: 'go', sort: 'oldest' });

    expect(seenUrl).toBe('/api/posts?page=2&limit=3&tag=go&sort=oldest');
    expect(seenInit?.method).toBeUndefined(); // GET carries no method override
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.totalPages).toBe(3);
      expect(result.value.items[0]?.id).toBe('p-7');
    }
  });

  it('creates with a JSON body POST and resolves the stored post (201)', async () => {
    let seenInit: RequestInit | undefined;
    vi.stubGlobal(
      'fetch',
      vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
        seenInit = init;
        return {
          ok: true,
          status: 201,
          headers: {},
          json: async () => wirePost,
          text: async () => JSON.stringify(wirePost),
        };
      }),
    );

    const result = await makeRepository().create({ title: 'A post from the spec', body: 'The stored body.', tags: ['go', 'api'] });

    expect(seenInit?.method).toBe('POST');
    expect(seenInit?.body).toBe('{"title":"A post from the spec","body":"The stored body.","tags":["go","api"]}');
    expect(result.ok).toBe(true);
    if (result.ok) expect(result.value.id).toBe('p-7');
  });

  it('replaces with a PUT to the item URL and resolves the replaced post', async () => {
    let seenUrl = '';
    let seenInit: RequestInit | undefined;
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        seenUrl = typeof input === 'string' ? input : input.toString();
        seenInit = init;
        return {
          ok: true,
          status: 200,
          headers: {},
          json: async () => wirePost,
          text: async () => JSON.stringify(wirePost),
        };
      }),
    );

    const result = await makeRepository().replace('p-2', { title: 'next', body: 'state', tags: [] });

    expect(seenUrl).toBe('/api/posts/p-2');
    expect(seenInit?.method).toBe('PUT');
    expect(result.ok).toBe(true);
  });

  it('deletes with a bodyless DELETE and resolves success on 204', async () => {
    stubApiFetch([{ url: '/api/posts/p-3', method: 'DELETE', status: 204 }]);

    const result = await makeRepository().remove('p-3');

    expect(result).toEqual({ ok: true, value: undefined });
  });

  it('guards the strict-stance 404 envelope into a Failed Result', async () => {
    stubApiFetch([
      {
        url: '/api/posts/p-3',
        method: 'DELETE',
        status: 404,
        body: { error: { code: 'not_found', message: 'Post "p-3" not found.', requestId: 'req-31' } },
      },
    ]);

    const result = await makeRepository().remove('p-3');

    expect(result).toEqual(
      failed(
        new ServerFailure('Post "p-3" not found.', {
          code: 'not_found',
          status: 404,
          requestId: 'req-31',
        }),
      ),
    );
  });

  it('guards the 422 issue envelope into a Failed Result (broken draft)', async () => {
    stubApiFetch([
      {
        url: '/api/posts',
        method: 'POST',
        status: 422,
        body: {
          error: {
            code: 'validation_failed',
            message: 'Invalid post body.',
            requestId: 'req-32',
            details: [{ field: 'title', rule: 'required', got: '""' }],
          },
        },
      },
    ]);

    const result = await makeRepository().create({ title: '', body: '', tags: [] });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('validation_failed');
      expect(result.failure.status).toBe(422);
      expect(result.failure.requestId).toBe('req-32');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().list({});

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // items entries missing every field — the DTO validator must reject.
      { url: '/api/posts', body: { items: [{ id: 'p-1' }], page: 1 } },
    ]);

    const result = await makeRepository().list({});

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
