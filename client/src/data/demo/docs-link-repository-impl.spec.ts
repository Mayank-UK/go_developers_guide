import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { DocsLinkApi } from './docs-link-api';
import { DocsLinkRepositoryImpl } from './docs-link-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * DocsLinkRepositoryImpl spec — fixture-driven (happy + failure): the
 * happy payload round-trips into the domain shape; network / envelope /
 * malformed-payload failures all resolve as Failed Results, never throws.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  spec: {
    title: 'Go Developers Guide — posts API',
    version: '1.0.0',
    openapiVersion: '3.1.0',
    specPath: 'server/api/openapi.yaml',
    specURL: '/api/openapi.yaml',
    servers: ['http://localhost:8100'],
    pathCount: 2,
    operationCount: 5,
    schemaCount: 6,
    validationIssue: 'the drift test walks live JSON against these schemas',
  },
  operations: [
    { id: 'listPosts', method: 'GET', path: '/api/posts', summary: 'One page of the posts collection' },
    { id: 'createPost', method: 'POST', path: '/api/posts', summary: 'Create a post from a full draft body' },
    { id: 'getPost', method: 'GET', path: '/api/posts/{id}', summary: 'Read one post by id' },
    { id: 'replacePost', method: 'PUT', path: '/api/posts/{id}', summary: 'Full replacement — the body is the whole next state' },
    { id: 'deletePost', method: 'DELETE', path: '/api/posts/{id}', summary: 'Remove a post under the strict stance' },
  ],
  schemas: [
    { name: 'Post', description: 'One post on the wire — the response shape of read, create, and replace.' },
    { name: 'PostPage', description: 'The list envelope — one page’s items plus the honest pagination metadata.' },
  ],
  serving: {
    specURL: '/api/openapi.yaml',
    contentType: 'application/yaml',
    servedVerbatim: true,
    swaggerStory: 'point Swagger UI at the URL and it renders the docs from the same bytes',
  },
  drift: [
    {
      check: 'list response ↔ PostPage',
      compares: 'GET /api/posts body: every key, type, and bound against the schema',
      settled: 'spec_drift_test.go drives the live handler and walks the JSON',
    },
  ],
  specText: 'openapi: 3.1.0\n',
};

function makeRepository(): DocsLinkRepositoryImpl {
  return new DocsLinkRepositoryImpl(new DocsLinkApi(createAppConfig()));
}

describe('DocsLinkRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/api/docs-link', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.spec.title).toBe('Go Developers Guide — posts API');
      expect(result.value.spec.operationCount).toBe(5);
      expect(result.value.operations[4]?.method).toBe('DELETE');
      expect(result.value.serving.servedVerbatim).toBe(true);
      expect(result.value.specText).toContain('openapi: 3.1.0');
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
        url: '/api/demo/api/docs-link',
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

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // Missing the spec facts — the DTO validator must reject it.
      { url: '/api/demo/api/docs-link', body: { operations: [] } },
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

  it('rejects an unknown HTTP method instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/api/docs-link',
        body: {
          ...demoPayload,
          operations: [{ id: 'x', method: 'FETCH', path: '/api/posts', summary: 's' }],
        },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });
});
