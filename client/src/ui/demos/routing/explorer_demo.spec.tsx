import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { RoutingExplorerDemo } from './explorer_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetRoutingExplorerDemoUseCase,
  ROUTING_EXPLORER_DEMO_USE_CASE,
} from '../../../domain/demo/get-routing-explorer-demo-usecase';
import type { RoutingExplorerDemoData } from '../../../domain/demo/routing-explorer';

/**
 * RoutingExplorerDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the carried route table renders with its shapes and handlers, probe
 * verdicts render with their decisive facts (matched pattern, captures,
 * Allow, redirect target) and reasoning, exhibits render as labeled
 * what-ifs, semantics facts render, and failures degrade to the shared
 * error panel with recovery.
 *
 * (The demo-registry binding test lands with the composition-root wiring —
 * this slice ships before demo-registry.tsx grows its lazy entry.)
 */

const fixture: RoutingExplorerDemoData = {
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
      reasoning: 'The wildcard captured the third segment, read with r.PathValue.',
    },
    {
      method: 'HEAD',
      path: '/api/content/routing?verbose=1&tag=go&tag=routing',
      outcome: 'matched',
      status: 200,
      matchedPattern: 'GET /api/content/{topicId}',
      queryValues: [
        { name: 'verbose', values: ['1'], source: 'query' },
        { name: 'tag', values: ['go', 'routing'], source: 'query' },
      ],
      reasoning: 'The query string never participates in matching.',
    },
    {
      method: 'DELETE',
      path: '/api/demo/scenario',
      outcome: 'method_not_allowed',
      status: 405,
      allow: ['GET', 'HEAD', 'PATCH'],
      reasoning: 'Allow unites both patterns and expands the implied HEAD.',
    },
    {
      method: 'GET',
      path: '/api/content/',
      outcome: 'not_found',
      status: 404,
      reasoning: 'There is no reverse-slash redirect.',
    },
  ],
  exhibits: [
    {
      id: 'precedence',
      title: 'Literal beats wildcard',
      note: 'The hypothetical literal beside the real wildcard.',
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
          reasoning: 'Most-specific-wins, literal over wildcard.',
        },
      ],
    },
    {
      id: 'subtree',
      title: 'The trailing-slash subtree and its 307',
      note: 'A pattern ending in / matches everything under the prefix.',
      patterns: [{ method: 'GET', pattern: '/assets/', kind: 'subtree' }],
      probes: [
        {
          method: 'GET',
          path: '/assets',
          outcome: 'redirect',
          status: 307,
          redirectTarget: '/assets/',
          reasoning: 'The subtree root without its slash answers 307.',
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

function stubUseCase(execute: () => Promise<Result<RoutingExplorerDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    ROUTING_EXPLORER_DEMO_USE_CASE,
    { execute } as unknown as GetRoutingExplorerDemoUseCase,
  );
}

describe('RoutingExplorerDemo', () => {
  it('renders loading first, then the full explorer', async () => {
    stubUseCase(async () => success(fixture));

    render(<RoutingExplorerDemo />);

    expect(screen.getByTestId('routing-explorer-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('re-routes')).toBeInTheDocument());

    // The carried route table: patterns, shapes, handlers.
    const table = screen.getByTestId('re-routes');
    expect(table).toHaveTextContent('/api/content/{topicId}');
    expect(table).toHaveTextContent('wildcard');
    expect(table).toHaveTextContent('presentation.NewContentTopicHandler');
    expect(table).toHaveTextContent('PATCH');
    expect(table).toHaveTextContent('/api/demo/scenario');

    // Probe verdicts with their decisive facts.
    const probes = screen.getByTestId('re-probes');
    expect(probes).toHaveTextContent('→ 405');
    expect(probes).toHaveTextContent('method_not_allowed');
    expect(probes).toHaveTextContent('allow: GET, HEAD, PATCH');
    expect(probes).toHaveTextContent('tag=[go, routing]');
    expect(probes).toHaveTextContent('topicId=routing');

    // Exhibits as labeled what-ifs; exhibit-only patterns are marked.
    const precedence = screen.getByTestId('re-exhibit-precedence');
    expect(precedence).toHaveTextContent('GET /api/content/new (exhibit)');
    expect(precedence).toHaveTextContent('Most-specific-wins');

    const subtree = screen.getByTestId('re-exhibit-subtree');
    expect(subtree).toHaveTextContent('location: /assets/');
    expect(subtree).toHaveTextContent('→ 307');

    // The verified semantics facts.
    const semantics = screen.getByTestId('re-semantics');
    expect(semantics).toHaveTextContent('GET also serves HEAD');
    expect(semantics).toHaveTextContent('subtree redirect: 307');
    expect(semantics).toHaveTextContent('no reverse-slash redirect');
  });

  it('degrades to the shared error panel on a Failed Result, with recovery', async () => {
    let calls = 0;
    stubUseCase(async () => {
      calls += 1;
      return calls === 1
        ? failed(
            new ServerFailure('The demo endpoint is down.', {
              code: 'service_unavailable',
              status: 503,
              requestId: 'req-31',
            }),
          )
        : success(fixture);
    });

    render(<RoutingExplorerDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the explorer renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('re-routes')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
