import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetContextCancellationDemoUseCase } from '../../../domain/demo/get-context-cancellation-demo-usecase';
import type { ContextCancellationDemoData, PropagationRow, WorkRow } from '../../../domain/demo/context-cancellation';

/**
 * ContextCancellationDemo — the goroutines-and-channels topic's live demo
 * (demo block id `conc:context`). It renders the payload of
 * GET /api/demo/conc/context, which the server computes by performing a
 * real cancellation choreography on every request: a parent context with
 * an armed 400ms deadline, three workers derived through a real context
 * tree (parent → alpha+beta, alpha → gamma) that each complete an
 * ack-proven work phase, then an explicit cancel whose propagation is
 * recorded structurally — generation, derived-from, recorded position,
 * cause — never as timings. A second beat lets a 25ms deadline fire on
 * its own (context.DeadlineExceeded, verified with errors.Is), and a
 * ledger closes the books: four goroutines joined, five contexts
 * cancelled, nothing leaked.
 *
 * Dependency flow mirrors the views: the hook resolves the use case from
 * the container by default (the getter the composition root registers
 * under), failures arrive as typed Failures inside Results, and `retry`
 * re-runs the load. Specs inject the use case through the component's
 * optional prop instead of the container.
 */

type ContextCancellationDemoStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ContextCancellationDemoData }
  | { state: 'error'; failure: Failure };

export function useContextCancellationDemo(
  useCase: GetContextCancellationDemoUseCase = container.getContextCancellationDemoUseCase,
): { status: ContextCancellationDemoStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ContextCancellationDemoStatus>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });

    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus({ state: 'ready', data: result.value });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function ContextCancellationDemo({
  useCase,
}: { useCase?: GetContextCancellationDemoUseCase } = {}) {
  const { status, retry } = useContextCancellationDemo(useCase);

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="conc-context-loading" className="text-body-sm text-on-surface-variant">
        performing the cancellation choreography…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the context-cancellation demo" failure={status.failure} onRetry={retry} />;
  }

  const { choreography, workPhase, propagation, deadlineBeat, ledger } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section
        data-testid="cc-choreography"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">The choreography</h4>
        <p className="text-body-sm text-on-surface">
          Deadline armed: <code className="font-mono text-secondary">{choreography.deadlineBudgetMs}ms</code>{' '}
          · work items per worker:{' '}
          <code className="font-mono text-secondary">{choreography.workItemsPerWorker}</code>
        </p>
        <p className="text-body-sm text-on-surface">
          Trigger: {choreography.trigger}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          {choreography.tree.map((edge) => `${edge.from} → ${edge.to}`).join('   ')}
        </p>
        <p className="text-body-sm text-on-surface-variant">{choreography.ordering}</p>
      </section>

      <section data-testid="cc-work-phase" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Work phase — completed before the cancel
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">worker</th>
                <th scope="col" className="px-sm py-xxs">generation</th>
                <th scope="col" className="px-sm py-xxs">items processed</th>
              </tr>
            </thead>
            <tbody>
              {workPhase.map((row) => (
                <WorkRowCells key={row.workerId} row={row} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="cc-propagation" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Propagation — the recorded observation order
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">#</th>
                <th scope="col" className="px-sm py-xxs">worker</th>
                <th scope="col" className="px-sm py-xxs">gen</th>
                <th scope="col" className="px-sm py-xxs">derived from</th>
                <th scope="col" className="px-sm py-xxs">cause</th>
              </tr>
            </thead>
            <tbody>
              {propagation.map((row) => (
                <PropagationRowCells key={row.workerId} row={row} />
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          Gamma is cancelled through alpha, not directly — derivation is transitive, and every
          cause reads context canceled.
        </p>
      </section>

      <section
        data-testid="cc-deadline"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">The beat nobody cancels</h4>
        <p className="text-body-sm text-on-surface">
          A {deadlineBeat.budgetMs}ms budget with no cancel call →{' '}
          <span data-testid="cc-deadline-outcome" className="font-mono text-secondary">
            {deadlineBeat.outcome}
          </span>
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          {deadlineBeat.cause} · errors.Is(…, context.DeadlineExceeded) ={' '}
          {deadlineBeat.deadlineExceeded ? 'true' : 'false'}
        </p>
      </section>

      <section
        data-testid="cc-ledger"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">The ledger</h4>
        <p className="font-mono text-caption text-on-surface">
          goroutines {ledger.goroutinesLaunched} launched / {ledger.goroutinesJoined} joined ·
          contexts {ledger.contextsCreated} created / {ledger.contextsCancelled} cancelled
        </p>
        <p data-testid="cc-ledger-outcome" className="text-body-sm text-secondary">
          {ledger.outcome}
        </p>
      </section>
    </div>
  );
}

function WorkRowCells({ row }: { row: WorkRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{row.workerId}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{row.generation}</td>
      <td className="px-sm py-xxs text-secondary">{row.itemsProcessed}</td>
    </tr>
  );
}

function PropagationRowCells({ row }: { row: PropagationRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface-variant">{row.observedPosition}</td>
      <td className="px-sm py-xxs text-on-surface">{row.workerId}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{row.generation}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{row.derivedFrom}</td>
      <td className="px-sm py-xxs text-secondary">{row.cause}</td>
    </tr>
  );
}

export default ContextCancellationDemo;
