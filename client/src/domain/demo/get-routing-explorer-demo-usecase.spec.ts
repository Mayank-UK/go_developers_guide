import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { RoutingExplorerDemoData } from './routing-explorer';
import type { RoutingExplorerRepository } from './routing-explorer-repository';
import { GetRoutingExplorerDemoUseCase } from './get-routing-explorer-demo-usecase';

/**
 * GetRoutingExplorerDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<RoutingExplorerDemoData>): RoutingExplorerRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: RoutingExplorerDemoData = {
  routes: [
    { method: 'GET', pattern: '/api/health', kind: 'exact', handler: 'presentation.NewHealthHandler' },
    {
      method: 'GET',
      pattern: '/api/content/{topicId}',
      kind: 'wildcard',
      handler: 'presentation.NewContentTopicHandler',
    },
  ],
  probes: [
    {
      method: 'GET',
      path: '/api/content/routing',
      outcome: 'matched',
      status: 200,
      matchedPattern: 'GET /api/content/{topicId}',
      pathValues: [{ name: 'topicId', values: ['routing'], source: 'path' }],
      reasoning: 'The wildcard captured the third segment.',
    },
    {
      method: 'DELETE',
      path: '/api/demo/scenario',
      outcome: 'method_not_allowed',
      status: 405,
      allow: ['GET', 'HEAD', 'PATCH'],
      reasoning: 'Two patterns share the path; Allow unions them.',
    },
  ],
  exhibits: [
    {
      id: 'precedence',
      title: 'Literal beats wildcard',
      note: 'A hypothetical literal beside the real wildcard.',
      patterns: [
        { method: 'GET', pattern: '/api/content/new', kind: 'exact' },
        {
          method: 'GET',
          pattern: '/api/content/{topicId}',
          kind: 'wildcard',
          handler: 'presentation.NewContentTopicHandler',
        },
      ],
      probes: [
        {
          method: 'GET',
          path: '/api/content/new',
          outcome: 'matched',
          status: 200,
          matchedPattern: 'GET /api/content/new',
          reasoning: 'Most specific wins.',
        },
      ],
    },
  ],
  semantics: {
    getAlsoServesHead: true,
    methodlessMatchesAll: true,
    subtreeRedirectStatus: 307,
    reverseSlashRedirect: false,
    matchesEscapedPath: true,
    pathValueIsUnescaped: true,
  },
};

describe('GetRoutingExplorerDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetRoutingExplorerDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetRoutingExplorerDemoUseCase(
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
