import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetRaceDemoUseCase,
  DB_RACE_DEMO_USE_CASE,
} from '../../../domain/demo/get-db-race-demo-usecase';
import type { RaceDemoData } from '../../../domain/demo/db-race';

/**
 * RaceDemo — the databases-sql topic's isolation demo (demo block id
 * `db:race`): two transactions stepped through their phases — the
 * invisible uncommitted write, the post-commit read, the FOR UPDATE wait
 * — with the serializable counterfactual beside them.
 */

type Status =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: RaceDemoData }
  | { state: 'error'; failure: Failure };

export function useRaceDemo(
  useCase: GetRaceDemoUseCase = container.resolve(DB_RACE_DEMO_USE_CASE),
): { status: Status; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<Status>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });
    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) setStatus({ state: 'ready', data: result.value });
      else setStatus({ state: 'error', failure: result.failure });
    });
    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function RaceDemo() {
  const { status, retry } = useRaceDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="race-loading" className="text-body-sm text-on-surface-variant">
        choreographing two transactions…
      </p>
    );
  }
  if (status.state === 'error') {
    return <ErrorPanel subject="the isolation demo" failure={status.failure} onRetry={retry} />;
  }

  const { isolation, scratchBefore, scratchAfter, steps, lockWaited, serializableNote } = status.data;
  return (
    <div className="flex flex-col gap-md" data-testid="race-ready">
      <section data-testid="race-header" className="flex flex-wrap items-baseline gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <span className="text-body-sm text-on-surface-variant">isolation:</span>
        <code className="font-mono text-body-sm text-secondary">{isolation}</code>
        <span className="ml-auto font-mono text-caption text-on-surface-variant">
          {scratchBefore} → {scratchAfter}
        </span>
      </section>

      <ol data-testid="race-steps" className="flex flex-col gap-xs">
        {steps.map((step, index) => (
          <li key={step.phase} className="flex flex-col gap-xxs rounded-md border border-outline px-sm py-xxs">
            <div className="flex flex-wrap items-baseline gap-xs">
              <span className="font-mono text-caption text-secondary">{index + 1}.</span>
              <span className="font-mono text-body-sm text-on-surface">{step.actor}</span>
              <code className="rounded bg-surface-tint px-xs py-xxs font-mono text-caption text-on-surface">{step.action}</code>
              <span className={`font-mono text-body-sm ${step.observed.includes('waited') ? 'text-error' : 'text-secondary'}`}>
                saw: {step.observed}
              </span>
            </div>
            <p className="text-caption text-on-surface-variant">{step.note}</p>
          </li>
        ))}
      </ol>

      <section data-testid="race-verdict" className="flex flex-col gap-xxs">
        <p className="text-body-sm text-on-surface">
          <span className={`rounded px-xs py-xxs font-mono text-caption ${lockWaited ? 'bg-secondary text-surface' : 'bg-surface-tint text-on-surface-variant'}`}>
            {lockWaited ? 'the reader WAITED for the row lock' : 'no lock wait observed'}
          </span>
        </p>
        <p className="text-body-sm text-on-surface-variant">{serializableNote}</p>
      </section>
    </div>
  );
}

export default RaceDemo;
