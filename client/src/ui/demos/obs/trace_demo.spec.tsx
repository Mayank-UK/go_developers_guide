import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ObsTraceDemo } from './trace_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetObsTraceDemoUseCase,
  OBS_TRACE_DEMO_USE_CASE,
} from '../../../domain/demo/get-obs-trace-demo-usecase';
import type { ObsTraceDemoData } from '../../../domain/demo/obs-trace';

/**
 * ObsTraceDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token. Behavior, not
 * implementation: the acceptance panel shows the adopted header, the
 * parsed trace id, and the response echo; the tree renders its spans
 * with layers and durations (depth driving indentation); the malformed
 * table renders the parser's real reasons; and failures degrade to
 * the shared error panel with recovery.
 */

const fixture: ObsTraceDemoData = {
  accepted: {
    inbound: '00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01',
    inboundOrigin: "the request's own header, adopted verbatim",
    version: '00',
    traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
    parentId: '00f067aa0ba902b7',
    flags: '01',
    sampled: true,
    tracestate: 'go=guide',
    responseEcho: '00-4bf92f3577b34da6a3ce929d0e0e4736-96a123b077498b49-01',
    note: 'the response carries the server span',
  },
  tree: [
    {
      name: 'GET /api/posts (traced walk)',
      layer: 'middleware',
      spanId: '96a123b077498b49',
      parentId: '00f067aa0ba902b7',
      traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
      depth: 0,
      startOffsetMs: 0,
      durationMs: 1.4,
      note: 'the graft point: the root parents on the inbound remote span',
    },
    {
      name: 'PostsListHandler.ServeHTTP',
      layer: 'presentation',
      spanId: 'aabbee0011223344',
      parentId: '96a123b077498b49',
      traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
      depth: 1,
      startOffsetMs: 0.2,
      durationMs: 1.1,
      note: 'the thin handler boundary',
    },
    {
      name: 'InMemoryRepository.List',
      layer: 'data',
      spanId: 'c1f0e2d3b4a59687',
      parentId: 'aabbee0011223344',
      traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
      depth: 3,
      startOffsetMs: 0.9,
      durationMs: 0.2,
      note: 'the leaf that answered',
    },
  ],
  malformed: [
    {
      header: '00-00000000000000000000000000000000-00f067aa0ba902b7-01',
      problem: 'traceparent …: trace-id must not be all zeros',
      handledAs: 'rejected by the parser; the middleware proceeds under a fresh root',
    },
    {
      header: 'ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01',
      problem: 'traceparent …: version ff is reserved',
      handledAs: 'rejected by the parser; the middleware proceeds under a fresh root',
    },
  ],
  sdk: 'what the OpenTelemetry SDK adds: samplers, exporters, resources',
  reasoning: 'one header became one tree',
};

function stubUseCase(execute: () => Promise<Result<ObsTraceDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    OBS_TRACE_DEMO_USE_CASE,
    { execute } as unknown as GetObsTraceDemoUseCase,
  );
}

describe('ObsTraceDemo', () => {
  it('renders loading first, then the acceptance panel, the span tree, and the malformed table', async () => {
    stubUseCase(async () => success(fixture));

    render(<ObsTraceDemo />);

    expect(screen.getByTestId('obs-trace-loading')).toBeInTheDocument();

    await waitForReady();
    const accepted = screen.getByTestId('obs-trace-accepted');
    expect(accepted).toHaveTextContent('4bf92f3577b34da6a3ce929d0e0e4736');
    expect(accepted).toHaveTextContent('adopted verbatim');
    expect(accepted).toHaveTextContent('(sampled)');
    expect(accepted).toHaveTextContent('go=guide');
    expect(accepted).toHaveTextContent('00-4bf92f3577b34da6a3ce929d0e0e4736-96a123b077498b49-01');

    const tree = screen.getByTestId('obs-trace-tree');
    const layers = ['middleware', 'presentation', 'data'];
    for (const [index, layer] of layers.entries()) {
      const span = screen.getByTestId(`obs-trace-span-${String(index + 1)}`);
      expect(span).toHaveTextContent(layer);
      expect(span).toHaveTextContent(fixture.tree[index]?.name ?? '');
    }
    expect(tree).toHaveTextContent('the graft point');

    const malformed = screen.getByTestId('obs-trace-malformed');
    expect(malformed).toHaveTextContent('all zeros');
    expect(malformed).toHaveTextContent('version ff is reserved');
    expect(malformed).toHaveTextContent('fresh root');

    expect(screen.getByTestId('obs-trace-sdk')).toHaveTextContent('OpenTelemetry SDK');
    expect(screen.getByTestId('obs-trace-reasoning')).toHaveTextContent('one header became one tree');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure('The trace round-trip could not be reached.', {
      code: 'network_error',
      status: 0,
      requestId: 'rid-trace-down',
    });
    stubUseCase(async () => failed(failure));

    render(<ObsTraceDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('obs-trace-ready')).toBeInTheDocument());
}
