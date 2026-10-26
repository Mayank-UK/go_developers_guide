import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ConcurrencyDemo } from './concurrency_demo';
import { demoRegistry } from '../../demos/demo-registry';
import { container, ContainerTokens } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import type { GetConcurrencyDemoUseCase } from '../../../domain/demo/get-concurrency-demo-usecase';
import type { ConcurrencyDemoData } from '../../../domain/demo/concurrency';

/**
 * ConcurrencyDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container, so the spec swaps the token
 * registration (the container's documented replace-a-singleton seam)
 * instead of stubbing fetch. Behavior, not implementation: the six payload
 * faces render, the deterministic beats and the observed beats are both
 * displayed honestly (a timer-won race's empty received, a worker's zero
 * tally), and failures degrade to the shared error panel with recovery.
 */

const fixture: ConcurrencyDemoData = {
  setup: {
    jobs: 8,
    workers: 3,
    cancelAfter: 5,
    strategy: 'worker pool: fan-out over one jobs channel, fan-in over one results channel, cancellation via select',
  },
  fanOut: [
    { jobId: 'job-01', payload: 'alpha', output: 'ALPHA#518' },
    { jobId: 'job-02', payload: 'bravo', output: 'BRAVO#538' },
    { jobId: 'job-03', payload: 'charlie', output: 'CHARLIE#728' },
  ],
  workerCounts: [
    { workerId: 'worker-1', jobsDone: 4 },
    { workerId: 'worker-2', jobsDone: 0 },
    { workerId: 'worker-3', jobsDone: 4 },
  ],
  cancellation: {
    jobsSubmitted: 8,
    resultsCollected: 5,
    jobsAbandoned: 3,
    workersStopped: 3,
    outcome: 'context cancelled after 5 of 8 results; 3 jobs abandoned, 3 workers returned',
  },
  timeouts: [
    { scenario: 'reply already waiting', producerDelayMs: 0, timeoutMs: 30, winner: 'reply', received: 'pong' },
    { scenario: 'reply arrives late', producerDelayMs: 200, timeoutMs: 30, winner: 'timer', received: '' },
  ],
  runtime: { gomaxprocs: 8, goroutinesBefore: 2, goroutinesDuring: 5, goroutinesAfter: 2 },
};

function stubUseCase(execute: () => Promise<Result<ConcurrencyDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    ContainerTokens.GetConcurrencyDemoUseCase,
    { execute } as unknown as GetConcurrencyDemoUseCase,
  );
}

describe('ConcurrencyDemo', () => {
  it('is bound in the demo registry under its demo-block id', () => {
    expect(demoRegistry['runtime:concurrency']).toBeDefined();
  });

  it('renders loading first, then all six payload faces', async () => {
    stubUseCase(async () => success(fixture));

    render(<ConcurrencyDemo />);

    expect(screen.getByTestId('concurrency-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('conc-setup')).toBeInTheDocument());

    // Setup: the configuration, identical on every call.
    expect(screen.getByTestId('conc-setup')).toHaveTextContent('8 jobs');
    expect(screen.getByTestId('conc-setup')).toHaveTextContent('cancel after 5 results');

    // Fan-out: pure transform rows in content order.
    expect(screen.getByTestId('conc-fanout')).toHaveTextContent('job-01');
    expect(screen.getByTestId('conc-fanout')).toHaveTextContent('ALPHA#518');

    // Worker tallies: the observed split, zero tally included, honestly.
    expect(screen.getByTestId('conc-workers')).toHaveTextContent('worker-2');
    expect(screen.getByTestId('conc-workers')).toHaveTextContent('0');

    // Cancellation: the balancing ledger and its story.
    expect(screen.getByTestId('conc-cancellation')).toHaveTextContent('3 abandoned');
    expect(screen.getByTestId('conc-cancellation')).toHaveTextContent('3 workers returned');

    // Timeouts: reply wins the rigged ready race, timer wins the late one,
    // and the empty received renders as data.
    expect(screen.getByTestId('conc-timeouts')).toHaveTextContent('reply');
    expect(screen.getByTestId('conc-timeouts')).toHaveTextContent('timer');
    expect(screen.getByTestId('conc-timeouts')).toHaveTextContent('pong');
    expect(screen.getByTestId('conc-timeouts')).toHaveTextContent('empty is data');

    // Runtime facts: GOMAXPROCS and the goroutine bracket.
    expect(screen.getByTestId('conc-runtime')).toHaveTextContent('GOMAXPROCS 8');
    expect(screen.getByTestId('conc-runtime')).toHaveTextContent('2 → 5 → 2');
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
              requestId: 'req-32',
            }),
          )
        : success(fixture);
    });

    render(<ConcurrencyDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the payload renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('conc-setup')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
