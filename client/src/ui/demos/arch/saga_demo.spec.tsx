import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ArchSagaDemo } from './saga_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetArchSagaDemoUseCase,
  ARCH_SAGA_DEMO_USE_CASE,
} from '../../../domain/demo/get-arch-saga-demo-usecase';
import type { ArchSagaDemoData } from '../../../domain/demo/arch-saga';

/**
 * ArchSagaDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration
 * instead of stubbing fetch. Behavior, not implementation: the script
 * panel renders the steps with their undo halves (empty undo rendered
 * as the externalized-fact note), each run renders its status, trigger
 * when present, ledger rows in order, the world log, the counts, and
 * only the failure run carries the replay panel; the semantics and
 * surface render; failures degrade to the shared error panel with
 * recovery.
 */

const fixture: ArchSagaDemoData = {
  mode: 'compensation ledger: three observed runs of the real patterns.SagaRunner',
  source: 'server/internal/domain/patterns/saga.go',
  honestyNote: 'No-invention pin: every count was counted from the ledgers.',
  steps: [
    {
      order: 0,
      name: 'reserve-inventory',
      forward: 'inventory: reserved 3× gopher-plush for order o-1001',
      undo: 'inventory: released 3× gopher-plush for order o-1001',
    },
    {
      order: 3,
      name: 'send-confirmation',
      forward: 'email: confirmation queued for ops@example.com for order o-1001',
      undo: '',
    },
  ],
  runs: [
    {
      id: 'success',
      title: 'All four steps commit',
      lead: 'the happy path',
      script: 'four steps, no injected failure',
      status: 'completed',
      ledger: [
        { seq: 1, step: '0 · reserve-inventory', phase: 'forward', outcome: 'executed', detail: 'inventory: reserved 3× gopher-plush for order o-1001' },
      ],
      worldLog: ['inventory: reserved 3× gopher-plush for order o-1001'],
      counts: [{ label: 'steps executed', value: 4 }],
      note: 'four executed rows, zero compensate rows',
    },
    {
      id: 'failure',
      title: 'Step 2 fails: compensated in reverse',
      lead: 'book-shipment refuses',
      script: 'failure injected at step 2',
      status: 'compensated',
      trigger: 'saga stopped at step 2 (book-shipment): carrier: no ground pickup window for order o-1001',
      ledger: [
        { seq: 1, step: '0 · reserve-inventory', phase: 'forward', outcome: 'executed', detail: 'inventory: reserved 3× gopher-plush for order o-1001' },
        { seq: 2, step: '2 · book-shipment', phase: 'forward', outcome: 'failed', detail: 'carrier: no ground pickup window for order o-1001' },
      ],
      worldLog: [
        'inventory: reserved 3× gopher-plush for order o-1001',
        'payments: refunded 4200 cents to tok_visa_4242 for order o-1001',
      ],
      counts: [
        { label: 'already-compensated no-ops on the replay', value: 2 },
        { label: 'steps compensated', value: 2 },
      ],
      replay: [
        { seq: 3, step: '1 · charge-card', phase: 'compensate', outcome: 'already-compensated', detail: 'compensating action not called again' },
      ],
      note: 'the replay records no-ops and calls nothing',
    },
  ],
  semantics: ['compensation runs in REVERSE over the steps that EXECUTED'],
  surface: [
    { id: 'SagaRunner', kind: 'type', role: 'the orchestrator over ordered steps' },
    { id: 'SagaStatus', kind: 'vocabulary', role: 'completed | compensated | cancelled' },
  ],
};

function stubUseCase(execute: () => Promise<Result<ArchSagaDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    ARCH_SAGA_DEMO_USE_CASE,
    { execute } as unknown as GetArchSagaDemoUseCase,
  );
}

describe('ArchSagaDemo', () => {
  it('renders loading first, then the script, both runs, semantics, and surface', async () => {
    stubUseCase(async () => success(fixture));

    render(<ArchSagaDemo />);

    expect(screen.getByTestId('arch-saga-loading')).toBeInTheDocument();

    await waitForReady();
    const script = screen.getByTestId('arch-saga-script');
    expect(script).toHaveTextContent('reserve-inventory');
    expect(script).toHaveTextContent('nothing to undo (an externalized fact, placed last)');
    expect(script).toHaveTextContent('server/internal/domain/patterns/saga.go');

    const successRun = screen.getByTestId('arch-saga-run-success');
    expect(successRun).toHaveTextContent('completed');
    expect(successRun).toHaveTextContent('steps executed');
    // A success run carries no trigger line and no replay panel.
    expect(screen.queryByTestId('arch-saga-trigger-success')).not.toBeInTheDocument();
    expect(screen.queryByTestId('arch-saga-run-replay-success')).not.toBeInTheDocument();

    const failureRun = screen.getByTestId('arch-saga-run-failure');
    expect(failureRun).toHaveTextContent('compensated');
    expect(screen.getByTestId('arch-saga-trigger-failure')).toHaveTextContent('book-shipment');
    expect(screen.getByTestId('arch-saga-row-2')).toHaveTextContent('failed');
    expect(screen.getByTestId('arch-saga-run-replay-failure')).toHaveTextContent('already-compensated');
    expect(failureRun).toHaveTextContent('2× already-compensated no-ops on the replay');

    expect(screen.getByTestId('arch-saga-semantics')).toHaveTextContent('REVERSE');
    expect(screen.getByTestId('arch-saga-surface')).toHaveTextContent('SagaRunner');
    expect(screen.getByTestId('arch-saga-honesty')).toHaveTextContent('No-invention pin');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure('The saga demo could not be reached.', {
      code: 'network_error',
      status: 0,
      requestId: 'rid-saga-down',
    });
    stubUseCase(async () => failed(failure));

    render(<ArchSagaDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('arch-saga-ready')).toBeInTheDocument());
}
