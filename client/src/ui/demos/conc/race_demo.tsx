import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetRaceDetectorDemoUseCase } from '../../../domain/demo/get-race-detector-demo-usecase';
import type { HygieneRow, RaceDetectorDemoData, SynchronizedRun } from '../../../domain/demo/race-detector';

/**
 * RaceDetectorDemo — the goroutines-and-channels topic's live demo (demo
 * block id `conc:race`). It renders the payload of
 * GET /api/demo/conc/race: the RACY counter taught, never run — its exact
 * code and the race detector's own verdict pinned verbatim from a
 * documented `go test -race` run (including the lost-update arithmetic:
 * the run's total came back 3900, not 5000) — beside BOTH fixes running
 * live: a mutex-guarded total and a channel fold, each landing on exactly
 * 5000 over 200 goroutines × 25 increments. A hygiene beat runs the
 * leaked-vs-done contrast for real: a delivered send versus a send
 * abandoned by select+default, with both goroutines joined first.
 *
 * Dependency flow mirrors the views: the hook resolves the use case from
 * the container by default (the getter the composition root registers
 * under), failures arrive as typed Failures inside Results, and `retry`
 * re-runs the load. Specs inject the use case through the component's
 * optional prop instead of the container.
 */

type RaceDetectorDemoStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: RaceDetectorDemoData }
  | { state: 'error'; failure: Failure };

export function useRaceDetectorDemo(
  useCase: GetRaceDetectorDemoUseCase = container.getRaceDetectorDemoUseCase,
): { status: RaceDetectorDemoStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<RaceDetectorDemoStatus>({ state: 'idle' });

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

export function RaceDetectorDemo({
  useCase,
}: { useCase?: GetRaceDetectorDemoUseCase } = {}) {
  const { status, retry } = useRaceDetectorDemo(useCase);

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="conc-race-loading" className="text-body-sm text-on-surface-variant">
        running the synchronized totals…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the race-detector demo" failure={status.failure} onRetry={retry} />;
  }

  const { race, mutex, channel, hygiene } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section data-testid="rd-exhibit" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The exhibit — pinned, never executed
        </h4>
        <p className="text-body-sm text-on-surface-variant">{race.explanation}</p>
        <pre className="overflow-x-auto rounded-md border border-outline bg-surface-tint px-sm py-xs font-mono text-caption text-on-surface">
          {race.code}
        </pre>
        <p className="text-body-sm text-on-surface">
          The toolchain's own verdict, captured from{' '}
          <code className="font-mono text-secondary">{race.command}</code>:
        </p>
        <pre
          data-testid="rd-verdict"
          className="overflow-x-auto rounded-md border border-outline bg-surface-tint px-sm py-xs font-mono text-caption text-error"
        >
          {race.verdict}
        </pre>
        <p className="text-body-sm text-on-surface-variant">{race.fix}</p>
      </section>

      <section data-testid="rd-mutex" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">Fix one — the mutex, live</h4>
        <RunCard run={mutex} testId="rd-mutex-exact" />
      </section>

      <section data-testid="rd-channel" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          Fix two — share memory by communicating, live
        </h4>
        <RunCard run={channel} testId="rd-channel-exact" />
      </section>

      <section data-testid="rd-hygiene" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Hygiene — leaked versus done
        </h4>
        <div className="flex flex-col gap-xs">
          {hygiene.map((row) => (
            <HygieneCard key={row.case} row={row} />
          ))}
        </div>
      </section>
    </div>
  );
}

function RunCard({ run, testId }: { run: SynchronizedRun; testId: string }) {
  return (
    <div className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
      <p className="text-body-sm text-on-surface">{run.strategy}</p>
      <p className="font-mono text-caption text-on-surface-variant">
        {run.goroutines} goroutines × {run.incrementsEach} increments = expected{' '}
        {run.expected}
      </p>
      <p className="font-mono text-body-sm">
        <span className="text-on-surface">observed {run.observed}</span>
        <span className="text-on-surface-variant"> → </span>
        {run.exact ? (
          <span data-testid={testId} className="text-secondary">
            exact
          </span>
        ) : (
          <span data-testid={testId} className="text-error">
            drifted
          </span>
        )}
      </p>
    </div>
  );
}

function HygieneCard({ row }: { row: HygieneRow }) {
  return (
    <div className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm">
      <p className="text-body-sm text-on-surface">{row.case}</p>
      <p className="font-mono text-caption">
        <span className={row.readerPresent ? 'text-secondary' : 'text-error'}>
          reader {row.readerPresent ? 'present' : 'absent'}
        </span>
        <span className="text-on-surface-variant"> · {row.outcome}</span>
      </p>
    </div>
  );
}

export default RaceDetectorDemo;
