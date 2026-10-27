import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { DebugStallDemo } from './stall_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetDebugStallDemoUseCase,
  DEBUG_STALL_DEMO_USE_CASE,
} from '../../../domain/demo/get-debug-stall-demo-usecase';
import type { DebugStallDemoData } from '../../../domain/demo/debug-stall';

/**
 * DebugStallDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token.
 * Behavior, not implementation: the workload panel states the
 * choreography; the capture panel states the provenance; the stalled
 * stack renders its frames with source lines and the created-by chain;
 * the contrast row renders the request's own goroutine from the same
 * capture; the endpoints panel points at the live /internal/ surface;
 * and failures degrade to the shared error panel with recovery.
 */

const fixture: DebugStallDemoData = {
  mode: 'runtime-derived: the stalled stack below was captured from THIS process',
  workload: {
    workers: 5,
    healthyDone: 4,
    wedge: 'one worker of the set blocks forever on a channel receive nobody serves',
  },
  capture: {
    profile: 'goroutine',
    goroutinesSeen: 9,
    captureMs: 0.31,
    captureTries: 1,
    curl: "curl -s 'localhost:8100/internal/debug/pprof/goroutine?debug=2'",
  },
  stalled: {
    id: 42,
    state: 'chan receive',
    wait: '1 minutes',
    frames: [
      {
        function: 'guide/internal/domain/demos.debugStallWedge(0x0)',
        location: 'server/internal/domain/demos/debug_stall.go:118',
      },
    ],
    createdBy: 'guide/internal/domain/demos.RunDebugStall',
    createdAt: 'server/internal/domain/demos/debug_stall.go:157',
  },
  contrast: {
    id: 1,
    state: 'running',
    frames: [
      {
        function: 'guide/internal/domain/demos.RunDebugStall(0x0)',
        location: 'server/internal/domain/demos/debug_stall.go:201',
      },
    ],
  },
  endpoints: [
    {
      path: '/internal/debug/pprof/goroutine?debug=2',
      profile: 'goroutine',
      note: 'every live goroutine with its full stack',
    },
    {
      path: '/internal/debug/pprof/profile?seconds=2',
      profile: 'cpu',
      note: 'a CPU profile sampled over the next N seconds',
    },
  ],
  reasoning: 'the runtime names the wedge — id, state, frames, and the created-by chain',
};

function stubUseCase(execute: () => Promise<Result<DebugStallDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    DEBUG_STALL_DEMO_USE_CASE,
    { execute } as unknown as GetDebugStallDemoUseCase,
  );
}

describe('DebugStallDemo', () => {
  it('renders loading first, then the workload, capture, stalled, contrast, and endpoints', async () => {
    stubUseCase(async () => success(fixture));

    render(<DebugStallDemo />);

    expect(screen.getByTestId('debug-stall-loading')).toBeInTheDocument();

    await waitForReady();
    const workload = screen.getByTestId('debug-stall-workload');
    expect(workload).toHaveTextContent('4/5 workers completed');
    expect(workload).toHaveTextContent('runtime-derived');

    const capture = screen.getByTestId('debug-stall-capture');
    expect(capture).toHaveTextContent('goroutine profile');
    expect(capture).toHaveTextContent('9 goroutines seen');
    expect(capture).toHaveTextContent('/internal/debug/pprof/goroutine');

    const stalled = screen.getByTestId('debug-stall-stalled');
    expect(stalled).toHaveTextContent('goroutine 42');
    expect(stalled).toHaveTextContent('chan receive');
    expect(stalled).toHaveTextContent('1 minutes');
    expect(stalled).toHaveTextContent('debugStallWedge');
    expect(stalled).toHaveTextContent('debug_stall.go:118');
    expect(stalled).toHaveTextContent('created by guide/internal/domain/demos.RunDebugStall');

    const contrast = screen.getByTestId('debug-stall-contrast');
    expect(contrast).toHaveTextContent('goroutine 1');
    expect(contrast).toHaveTextContent('running');

    const endpoints = screen.getByTestId('debug-stall-endpoints');
    expect(endpoints).toHaveTextContent('/internal/debug/pprof/profile?seconds=2');

    expect(screen.getByTestId('debug-stall-reasoning')).toHaveTextContent('created-by chain');
  });

  it('renders the honest "(not found)" empty state without inventing frames', async () => {
    stubUseCase(async () =>
      success({
        ...fixture,
        stalled: { ...fixture.stalled, state: '(not found)', wait: undefined, frames: [] },
      }),
    );

    render(<DebugStallDemo />);
    await waitForReady();

    const stalled = screen.getByTestId('debug-stall-stalled');
    expect(stalled).toHaveTextContent('(not found)');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure(
      "The stall demo could not find its wedged goroutine on the process's own profile; no stack was invented to cover for it.",
      { code: 'internal', status: 500, requestId: 'rid-stall-lost' },
    );
    stubUseCase(async () => failed(failure));

    render(<DebugStallDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('no stack was invented');

    // Retry re-runs the load through the same use case seam — a fresh
    // staging, since every GET wedges anew.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('debug-stall-ready')).toBeInTheDocument());
}
