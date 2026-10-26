import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { DocsLinkDemoData } from './docs-link';
import type { DocsLinkRepository } from './docs-link-repository';
import { GetDocsLinkDemoUseCase } from './get-docs-link-demo-usecase';

/**
 * GetDocsLinkDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<DocsLinkDemoData>): DocsLinkRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: DocsLinkDemoData = {
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
    { name: 'ErrorBody', description: "The envelope's payload: taxonomy bucket, human sentence, join key, structured context." },
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
  specText: 'openapi: 3.1.0\ninfo:\n  title: Go Developers Guide — posts API\n',
};

describe('GetDocsLinkDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetDocsLinkDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetDocsLinkDemoUseCase(
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
