import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { ReliabilityBreakerDemo } from './breaker_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import { stubApiFetch } from '../../../test/fetch_stub';
import {
  GetReliabilityBreakerDemoUseCase,
  RELIABILITY_BREAKER_DEMO_USE_CASE,
} from '../../../domain/demo/get-reliability-breaker-demo-usecase';
import type { ReliabilityBreakerDemoData } from '../../../domain/demo/reliability-breaker';

/**
 * ReliabilityBreakerDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token.
 * Behavior, not implementation: the embedded scenario control, the
 * scenario toggle and persistent breaker state render with config,
 * counters, and the transition ring; the walk renders attempt-by-attempt
 * state journeys with measured elapsed figures; the retry series renders
 * its bounded attempts with measured waits; the re-walk button drives a
 * fresh execute through the same persistent server-side breaker; and
 * failures degrade to the shared error panel with recovery.
 */

const fixture: ReliabilityBreakerDemoData = {
  mode: 'process-lifetime breaker: one shared circuit breaker protects the dependency',
  scenario: {
    mode: 'error',
    getEndpoint: '/api/demo/scenario',
    patchEndpoint: '/api/demo/scenario',
    modes: ['none', 'error', 'latency'],
    note: 'the runtime toggle the dependency honors on every call',
  },
  breaker: {
    state: 'open',
    stateAgeMs: 1400,
    config: {
      failureThreshold: 3,
      cooldownMs: 2000,
      callTimeoutMs: 120,
      walkAttempts: 4,
      halfOpenProbes: 1,
      retryAttempts: 3,
      retryBaseDelayMs: 40,
      retryJitterMs: 20,
    },
    counters: { allowed: 9, fastFailed: 4, succeeded: 5, failed: 4 },
    probeInFlight: false,
    transitions: [
      { at: '2026-10-28T09:30:00Z', from: 'closed', to: 'open', cause: '3 consecutive failures — threshold reached' },
      {
        at: '2026-10-28T09:30:02Z',
        from: 'open',
        to: 'half-open',
        cause: 'cooldown elapsed — probing the dependency',
      },
    ],
  },
  walk: {
    mode: 'error',
    rows: [
      {
        index: 1,
        stateBefore: 'half-open',
        action: 'call',
        outcome: 'injected error',
        detail: 'reliability demo: the scenario armed the dependency to fail',
        elapsedMs: 0.021,
        stateAfter: 'open',
      },
      {
        index: 2,
        stateBefore: 'open',
        action: 'fast-fail',
        outcome: 'refused',
        detail: 'open — fast-fail; 1.4s of the 2s cooldown remains',
        elapsedMs: 0.004,
        stateAfter: 'open',
      },
    ],
    note: 'the threshold tripped on attempt 1',
  },
  retry: {
    mode: 'error',
    succeeded: false,
    rows: [
      { attempt: 1, outcome: 'injected error', elapsedMs: 0.02, backoffMs: 41.25 },
      { attempt: 2, outcome: 'injected error', elapsedMs: 0.02, backoffMs: 80.5 },
      { attempt: 3, outcome: 'injected error', elapsedMs: 0.02, backoffMs: 0 },
    ],
    totalElapsedMs: 121.9,
    note: 'the series exhausted its 3 attempts',
  },
  reasoning: 'the breaker state persists across requests',
};

/** The embedded scenario panel's mount-time GET; stubbed so every render is deterministic. */
const scenario = { url: '/api/demo/scenario', body: { dataMode: 'fake', injectFailure: 'none' } };

function stubUseCase(execute: () => Promise<Result<ReliabilityBreakerDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    RELIABILITY_BREAKER_DEMO_USE_CASE,
    { execute } as unknown as GetReliabilityBreakerDemoUseCase,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('ReliabilityBreakerDemo', () => {
  it('renders loading first, then the control, scenario, breaker, walk, and retry panels', async () => {
    stubUseCase(async () => success(structuredClone(fixture)));
    stubApiFetch([scenario]);

    render(<ReliabilityBreakerDemo />);

    expect(screen.getByTestId('reliability-breaker-loading')).toBeInTheDocument();

    await screen.findByTestId('reliability-breaker-ready');

    // The embedded scenario control — the demo's own fake/live + failure
    // toggle, so the page drives standalone.
    expect(await screen.findByTestId('scenario-panel')).toBeInTheDocument();
    expect(screen.getByLabelText('Data mode')).toBeInTheDocument();
    expect(screen.getByLabelText('Inject failure')).toBeInTheDocument();

    const scenarioPanel = screen.getByTestId('reliability-breaker-scenario');
    expect(scenarioPanel).toHaveTextContent('scenario · error');
    expect(scenarioPanel).toHaveTextContent('/api/demo/scenario');
    expect(scenarioPanel).toHaveTextContent('none | error | latency');

    const breaker = screen.getByTestId('reliability-breaker-state');
    expect(breaker).toHaveTextContent('breaker · open · held for 1.4s');
    expect(breaker).toHaveTextContent('threshold 3 · cooldown 2s · deadline 120ms');
    expect(breaker).toHaveTextContent('1 half-open probe');
    expect(breaker).toHaveTextContent('allowed 9 · succeeded 5 · failed 4 · fast-failed 4');
    // The transition ring renders newest first, causes verbatim.
    expect(breaker).toHaveTextContent('open → half-open — cooldown elapsed');
    expect(breaker).toHaveTextContent('closed → open — 3 consecutive failures');

    const walk = screen.getByTestId('reliability-breaker-walk');
    expect(walk).toHaveTextContent('#1 half-open → open');
    expect(walk).toHaveTextContent('call · injected error · 0.02ms');
    expect(walk).toHaveTextContent('#2 open → open');
    expect(walk).toHaveTextContent('fast-fail · refused · 0.00ms');
    expect(walk).toHaveTextContent('cooldown remains');
    expect(walk).toHaveTextContent('the threshold tripped on attempt 1');

    const retry = screen.getByTestId('reliability-breaker-retry');
    expect(retry).toHaveTextContent('retry series · exhausted · total 122ms');
    expect(retry).toHaveTextContent('retry 1 · injected error · 0.02ms · wait 41.25ms');
    expect(retry).toHaveTextContent('retry 3 · injected error · 0.02ms · final attempt');

    expect(screen.getByTestId('reliability-breaker-reasoning')).toHaveTextContent(
      'persists across requests',
    );
  });

  it('renders the honest no-transitions-yet line for a breaker that never tripped', async () => {
    stubUseCase(async () =>
      success({
        ...structuredClone(fixture),
        breaker: {
          ...fixture.breaker,
          state: 'closed',
          stateAgeMs: 45000,
          transitions: [],
          counters: { allowed: 4, fastFailed: 0, succeeded: 4, failed: 0 },
        },
      }),
    );
    stubApiFetch([scenario]);

    render(<ReliabilityBreakerDemo />);

    const breaker = await screen.findByTestId('reliability-breaker-state');
    expect(breaker).toHaveTextContent('breaker · closed · held for 45s');
    expect(breaker).toHaveTextContent('no state changes yet');
  });

  it('re-walks through the same use case when the button fires', async () => {
    const execute = vi.fn(async () => success(structuredClone(fixture)));
    stubUseCase(execute);
    stubApiFetch([scenario]);

    render(<ReliabilityBreakerDemo />);
    await screen.findByTestId('reliability-breaker-ready');
    expect(execute).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByTestId('reliability-breaker-rewalk'));

    await waitFor(() => expect(execute).toHaveBeenCalledTimes(2));
    expect(await screen.findByTestId('reliability-breaker-ready')).toBeInTheDocument();
  });

  it('degrades to the shared error panel with recovery', async () => {
    stubUseCase(async () =>
      failed(new ServerFailure('the breaker demo endpoint is unreachable')),
    );
    stubApiFetch([scenario]);

    render(<ReliabilityBreakerDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('the reliability breaker demo');
    expect(panel).toHaveTextContent('unreachable');

    const recover = vi.fn(async () => success(structuredClone(fixture)));
    container.register(
      RELIABILITY_BREAKER_DEMO_USE_CASE,
      { execute: recover } as unknown as GetReliabilityBreakerDemoUseCase,
    );

    await fireEvent.click(screen.getByRole('button', { name: /try again/i }));
    expect(await screen.findByTestId('reliability-breaker-ready')).toBeInTheDocument();
    // The recovered view re-mounts the embedded scenario control.
    expect(await screen.findByTestId('scenario-panel')).toBeInTheDocument();
  });
});
