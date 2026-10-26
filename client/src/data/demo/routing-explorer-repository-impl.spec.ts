import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { RoutingExplorerApi } from './routing-explorer-api';
import { RoutingExplorerRepositoryImpl } from './routing-explorer-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * RoutingExplorerRepositoryImpl spec — fixture-driven (happy + failure):
 * the happy payload round-trips into the domain shape; network / envelope /
 * malformed-payload failures all resolve as Failed Results, never throws.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  routes: [
    { method: 'GET', pattern: '/api/health', kind: 'exact', handler: 'presentation.NewHealthHandler' },
    {
      method: 'GET',
      pattern: '/api/content/{topicId}',
      kind: 'wildcard',
      handler: 'presentation.NewContentTopicHandler',
    },
    { method: 'GET', pattern: '/api/demo/scenario', kind: 'exact', handler: 'presentation.NewScenarioGetHandler' },
    { method: 'PATCH', pattern: '/api/demo/scenario', kind: 'exact', handler: 'presentation.NewScenarioPatchHandler' },
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
      method: 'GET',
      path: '/api/content/routing?verbose=1&tag=go&tag=routing',
      outcome: 'matched',
      status: 200,
      matchedPattern: 'GET /api/content/{topicId}',
      queryValues: [
        { name: 'verbose', values: ['1'], source: 'query' },
        { name: 'tag', values: ['go', 'routing'], source: 'query' },
      ],
      reasoning: 'The query never participates in matching.',
    },
    {
      method: 'DELETE',
      path: '/api/demo/scenario',
      outcome: 'method_not_allowed',
      status: 405,
      allow: ['GET', 'HEAD', 'PATCH'],
      reasoning: 'Allow unites both patterns and expands the implied HEAD.',
    },
    { method: 'GET', path: '/api/content/a/b', outcome: 'not_found', status: 404, reasoning: 'Single-segment wildcard.' },
  ],
  exhibits: [
    {
      id: 'subtree',
      title: 'The trailing-slash subtree and its 307',
      note: 'A pattern ending in / matches everything under the prefix.',
      patterns: [{ method: 'GET', pattern: '/assets/', kind: 'subtree' }],
      probes: [
        { method: 'GET', path: '/assets', outcome: 'redirect', status: 307, redirectTarget: '/assets/', reasoning: '307 to the slashed form.' },
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

function makeRepository(): RoutingExplorerRepositoryImpl {
  return new RoutingExplorerRepositoryImpl(new RoutingExplorerApi(createAppConfig()));
}

describe('RoutingExplorerRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/routing/explorer', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.routes).toHaveLength(4);
      expect(result.value.routes[1]?.kind).toBe('wildcard');
      expect(result.value.probes[2]?.allow).toEqual(['GET', 'HEAD', 'PATCH']);
      expect(result.value.probes[1]?.queryValues?.[1]?.values).toEqual(['go', 'routing']);
      expect(result.value.semantics.subtreeRedirectStatus).toBe(307);
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
        url: '/api/demo/routing/explorer',
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
      // Missing the probe verdicts — the DTO validator must reject it.
      { url: '/api/demo/routing/explorer', body: { routes: demoPayload.routes } },
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

  it('rejects an unknown outcome vocabulary instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/routing/explorer',
        body: {
          ...demoPayload,
          probes: [{ method: 'GET', path: '/x', outcome: 'teapot', status: 418, reasoning: 'r' }],
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
