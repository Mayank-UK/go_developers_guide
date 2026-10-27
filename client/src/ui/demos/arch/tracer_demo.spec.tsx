import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ArchTracerDemo } from './tracer_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetArchTracerDemoUseCase,
  ARCH_TRACER_DEMO_USE_CASE,
} from '../../../domain/demo/get-arch-tracer-demo-usecase';
import type { ArchTracerDemoData } from '../../../domain/demo/arch-tracer';

/**
 * ArchTracerDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration
 * instead of stubbing fetch. Behavior, not implementation: the timeline
 * renders its hops in order with layers, files, symbols, and durations,
 * the leaf and flip note render, the counterexample renders as clearly
 * labeled data, and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: ArchTracerDemoData = {
  trace: {
    request: 'GET /api/posts?limit=3&sort=newest',
    dataMode: 'fake',
    leaf: 'InMemoryRepository.List — server/internal/data/posts/inmemory.go',
    flipNote: 'PATCH /api/demo/scenario {"dataMode":"live"} — reload for the other leaf',
    hops: [
      {
        order: 1,
        layer: 'middleware',
        file: 'server/internal/presentation/middleware.go',
        symbol: 'Chain(RequestID → RequestLogger → Recover → EnvelopeMuxErrors)',
        note: 'the documented order',
        durationMs: 0.35,
      },
      {
        order: 2,
        layer: 'presentation',
        file: 'server/internal/presentation/posts.go',
        symbol: 'PostsListHandler.ServeHTTP',
        note: 'parse, normalize, delegate, map',
        durationMs: 0.3,
      },
      {
        order: 3,
        layer: 'domain',
        file: 'server/internal/domain/posts/usecases.go',
        symbol: 'ListPostsUseCase.Execute',
        note: 'a one-line delegation over the port',
        durationMs: 0.14,
      },
      {
        order: 4,
        layer: 'data',
        file: 'server/internal/data/posts/inmemory.go',
        symbol: 'InMemoryRepository.List',
        note: 'the leaf that answered',
        durationMs: 0.11,
      },
    ],
    status: 200,
    items: 3,
    elapsedMs: 0.36,
    requestId: 'rid-arch-1',
    note: 'every hop is an execution that just happened',
  },
  violation: {
    label: 'counterexample — not code this repo compiles',
    snippet: 'func (h *BadPostsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { … }',
    why: 'the handler owns four jobs the architecture gives to four places',
    caughtBy: 'the import graph: one grep away',
    honest: 'no production presentation file imports the data layer',
  },
  reasoning: 'one request, four layers, every hop measured',
};

function stubUseCase(execute: () => Promise<Result<ArchTracerDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    ARCH_TRACER_DEMO_USE_CASE,
    { execute } as unknown as GetArchTracerDemoUseCase,
  );
}

describe('ArchTracerDemo', () => {
  it('renders loading first, then the timeline with every hop and the counterexample', async () => {
    stubUseCase(async () => success(fixture));

    render(<ArchTracerDemo />);

    expect(screen.getByTestId('arch-tracer-loading')).toBeInTheDocument();

    await waitForReady();
    const timeline = screen.getByTestId('arch-tracer-timeline');
    expect(timeline).toHaveTextContent('GET /api/posts?limit=3&sort=newest');
    expect(timeline).toHaveTextContent('mode fake');
    expect(timeline).toHaveTextContent('200 with 3 items');
    expect(timeline).toHaveTextContent('InMemoryRepository.List');

    const layers = ['middleware', 'presentation', 'domain', 'data'] as const;
    for (const [index, layer] of layers.entries()) {
      const hop = screen.getByTestId(`arch-tracer-hop-${String(index + 1)}`);
      expect(hop).toHaveTextContent(layer);
      expect(hop).toHaveTextContent(fixture.trace.hops[index]?.file ?? '');
      expect(hop).toHaveTextContent(fixture.trace.hops[index]?.symbol ?? '');
    }
    expect(timeline).toHaveTextContent('PATCH /api/demo/scenario');

    const violation = screen.getByTestId('arch-tracer-violation');
    expect(violation).toHaveTextContent('counterexample — not code this repo compiles');
    expect(violation).toHaveTextContent('BadPostsHandler');
    expect(violation).toHaveTextContent('no production presentation file imports the data layer');

    expect(screen.getByTestId('arch-tracer-reasoning')).toHaveTextContent('every hop measured');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure('The architecture tracer could not be reached.', {
      code: 'network_error',
      status: 0,
      requestId: 'rid-arch-down',
    });
    stubUseCase(async () => failed(failure));

    render(<ArchTracerDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('arch-tracer-ready')).toBeInTheDocument());
}
