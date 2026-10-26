import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetConcurrencyDemoUseCase } from '../../../domain/demo/get-concurrency-demo-usecase';
import type {
  ConcurrencyDemoData,
  JobRow,
  RuntimeFacts,
  TimeoutBeat,
  WorkerRow,
} from '../../../domain/demo/concurrency';

/**
 * ConcurrencyDemo — the go-runtime topic's live demo (demo block id
 * `runtime:concurrency`). It renders the payload of
 * GET /api/demo/runtime/concurrency, which the server computes by RUNNING
 * a real worker pool on every request: three goroutines fan out over
 * eight jobs and fan results back in through channels; a second run
 * cancels mid-stream after five counted results; two select races pit a
 * reply against a timer. Counts, ids, and outcomes never move between
 * calls; the per-worker tally and the goroutine counts are what the
 * runtime actually did, this run.
 *
 * Dependency flow mirrors the views: the hook resolves the use case from
 * the container (its default parameter is the spec seam — tests substitute
 * fakes through the container token instead), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load.
 */

type ConcurrencyDemoStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ConcurrencyDemoData }
  | { state: 'error'; failure: Failure };

export function useConcurrencyDemo(
  useCase: GetConcurrencyDemoUseCase = container.getConcurrencyDemoUseCase,
): { status: ConcurrencyDemoStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ConcurrencyDemoStatus>({ state: 'idle' });

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

export function ConcurrencyDemo() {
  const { status, retry } = useConcurrencyDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="concurrency-loading" className="text-body-sm text-on-surface-variant">
        running the worker pool…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the concurrency demo" failure={status.failure} onRetry={retry} />;
  }

  const { setup, fanOut, workerCounts, cancellation, timeouts, runtime } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section data-testid="conc-setup" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The choreography, every run the same
        </h4>
        <p className="text-body-sm text-on-surface">
          {setup.jobs} jobs · {setup.workers} workers · cancel after {setup.cancelAfter} results —{' '}
          <span className="text-on-surface-variant">{setup.strategy}</span>
        </p>
      </section>

      <section data-testid="conc-fanout" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Fan-out → fan-in — every job, in content order
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">job</th>
                <th scope="col" className="px-sm py-xxs">payload</th>
                <th scope="col" className="px-sm py-xxs">output</th>
              </tr>
            </thead>
            <tbody>
              {fanOut.map((row) => (
                <JobRowCells key={row.jobId} row={row} />
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          Channels deliver in completion order; the payload reports content
          order, sorted by job id — the transform is pure, so these rows
          never move between runs.
        </p>
      </section>

      <section data-testid="conc-workers" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Worker tallies — what the scheduler actually did
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">worker</th>
                <th scope="col" className="px-sm py-xxs">jobs handled</th>
              </tr>
            </thead>
            <tbody>
              {workerCounts.map((row) => (
                <WorkerRowCells key={row.workerId} row={row} />
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          The one varying fact: real workers race for real jobs, and this
          run's split is whatever the race produced — the rows always sum
          to eight.
        </p>
      </section>

      <section data-testid="conc-cancellation" className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Cancellation, mid-stream
        </h4>
        <p className="font-mono text-body-sm text-on-surface">
          {cancellation.jobsSubmitted} submitted · {cancellation.resultsCollected} collected ·{' '}
          <span className="text-error">{cancellation.jobsAbandoned} abandoned</span> ·{' '}
          {cancellation.workersStopped} workers returned
        </p>
        <p className="text-body-sm text-on-surface-variant">{cancellation.outcome}</p>
        <p className="text-body-sm text-on-surface-variant">
          The ledger must balance: collected + abandoned == submitted,
          every run.
        </p>
      </section>

      <section data-testid="conc-timeouts" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          select — reply vs timer, rigged by configuration
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">scenario</th>
                <th scope="col" className="px-sm py-xxs">producer</th>
                <th scope="col" className="px-sm py-xxs">timer</th>
                <th scope="col" className="px-sm py-xxs">winner</th>
                <th scope="col" className="px-sm py-xxs">received</th>
              </tr>
            </thead>
            <tbody>
              {timeouts.map((beat) => (
                <TimeoutRowCells key={beat.scenario} beat={beat} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="conc-runtime" className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          The runtime, on itself
        </h4>
        <RuntimeFactsRow facts={runtime} />
      </section>
    </div>
  );
}

function JobRowCells({ row }: { row: JobRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{row.jobId}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{row.payload}</td>
      <td className="px-sm py-xxs text-secondary">{row.output}</td>
    </tr>
  );
}

function WorkerRowCells({ row }: { row: WorkerRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{row.workerId}</td>
      <td className="px-sm py-xxs text-secondary">{row.jobsDone}</td>
    </tr>
  );
}

function TimeoutRowCells({ beat }: { beat: TimeoutBeat }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{beat.scenario}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{beat.producerDelayMs}ms</td>
      <td className="px-sm py-xxs text-on-surface-variant">{beat.timeoutMs}ms</td>
      <td className="px-sm py-xxs">
        {beat.winner === 'reply' ? (
          <span className="text-secondary">reply</span>
        ) : (
          <span className="text-error">timer</span>
        )}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">
        {beat.received === '' ? '— (empty is data)' : beat.received}
      </td>
    </tr>
  );
}

function RuntimeFactsRow({ facts }: { facts: RuntimeFacts }) {
  return (
    <p className="font-mono text-body-sm text-on-surface">
      GOMAXPROCS {facts.gomaxprocs} · goroutines {facts.goroutinesBefore} →{' '}
      {facts.goroutinesDuring} → {facts.goroutinesAfter}
      <span className="text-on-surface-variant">
        {' '}
        (before → three parked workers → after the join)
      </span>
    </p>
  );
}

export default ConcurrencyDemo;
