import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ContextCancellationDemo } from './context_demo';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import type { GetContextCancellationDemoUseCase } from '../../../domain/demo/get-context-cancellation-demo-usecase';
import type { ContextCancellationDemoData } from '../../../domain/demo/context-cancellation';

/**
 * ContextCancellationDemo spec — fixture-driven (happy + failure). The
 * component takes its use case through an optional prop (the container's
 * token wiring for this demo lands with the wave's composition-root
 * change, so the spec injects directly instead of stubbing the
 * container). Behavior, not implementation: all five payload faces
 * render, the propagation table shows the recorded order and gamma's
 * transitive derivation, and failures degrade to the shared error panel
 * with recovery.
 */

const fixture: ContextCancellationDemoData = {
  choreography: {
    deadlineBudgetMs: 400,
    workItemsPerWorker: 2,
    trigger: 'explicit cancel of the parent, before the armed deadline',
    tree: [
      { from: 'parent', to: 'alpha' },
      { from: 'parent', to: 'beta' },
      { from: 'alpha', to: 'gamma' },
    ],
    ordering:
      'recorded behind per-generation barriers, ids sorted within a generation — the guarantee is that every descendant observes Done()',
  },
  workPhase: [
    { workerId: 'alpha', generation: 1, itemsProcessed: 2 },
    { workerId: 'beta', generation: 1, itemsProcessed: 2 },
    { workerId: 'gamma', generation: 2, itemsProcessed: 2 },
  ],
  propagation: [
    { workerId: 'alpha', generation: 1, derivedFrom: 'parent', observedPosition: 1, cause: 'context canceled' },
    { workerId: 'beta', generation: 1, derivedFrom: 'parent', observedPosition: 2, cause: 'context canceled' },
    { workerId: 'gamma', generation: 2, derivedFrom: 'alpha', observedPosition: 3, cause: 'context canceled' },
  ],
  deadlineBeat: {
    budgetMs: 25,
    cancelCalled: false,
    outcome: 'deadline exceeded',
    cause: 'context deadline exceeded',
    deadlineExceeded: true,
  },
  ledger: {
    goroutinesLaunched: 4,
    goroutinesJoined: 4,
    contextsCreated: 5,
    contextsCancelled: 5,
    outcome: 'no goroutines leaked, every context cancelled',
  },
};

function stubUseCase(execute: () => Promise<Result<ContextCancellationDemoData>>): GetContextCancellationDemoUseCase {
  return { execute } as unknown as GetContextCancellationDemoUseCase;
}

describe('ContextCancellationDemo', () => {
  it('renders loading first, then all five payload faces', async () => {
    render(<ContextCancellationDemo useCase={stubUseCase(async () => success(fixture))} />);

    expect(screen.getByTestId('conc-context-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('cc-choreography')).toBeInTheDocument());

    // Choreography: the armed budget, the trigger, the tree as edges.
    expect(screen.getByTestId('cc-choreography')).toHaveTextContent('400ms');
    expect(screen.getByTestId('cc-choreography')).toHaveTextContent('parent → alpha');
    expect(screen.getByTestId('cc-choreography')).toHaveTextContent('alpha → gamma');

    // Work phase: every worker completed its ack-proven share.
    expect(screen.getByTestId('cc-work-phase')).toHaveTextContent('gamma');
    expect(screen.getByTestId('cc-work-phase')).toHaveTextContent('2');

    // Propagation: the recorded order and gamma's transitive derivation.
    expect(screen.getByTestId('cc-propagation')).toHaveTextContent('derived from');
    expect(screen.getByTestId('cc-propagation')).toHaveTextContent('alpha');
    expect(screen.getByTestId('cc-propagation')).toHaveTextContent('context canceled');

    // Deadline beat: the never-cancelled context exceeded on its own.
    expect(screen.getByTestId('cc-deadline-outcome')).toHaveTextContent('deadline exceeded');
    expect(screen.getByTestId('cc-deadline')).toHaveTextContent('context.DeadlineExceeded) = true');

    // Ledger: the leak-free books, stated.
    expect(screen.getByTestId('cc-ledger')).toHaveTextContent('4 launched / 4 joined');
    expect(screen.getByTestId('cc-ledger-outcome')).toHaveTextContent('no goroutines leaked');
  });

  it('degrades to the shared error panel on a Failed Result, with recovery', async () => {
    let calls = 0;
    render(
      <ContextCancellationDemo
        useCase={stubUseCase(async () => {
          calls += 1;
          return calls === 1
            ? failed(
                new ServerFailure('The demo endpoint is down.', {
                  code: 'service_unavailable',
                  status: 503,
                  requestId: 'req-61',
                }),
              )
            : success(fixture);
        })}
      />,
    );

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the payload renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('cc-propagation')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
