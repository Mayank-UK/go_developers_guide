import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetMigrationsDemoUseCase,
  DB_MIGRATIONS_DEMO_USE_CASE,
} from '../../../domain/demo/get-db-migrations-demo-usecase';
import type { MigrationsDemoData } from '../../../domain/demo/db-migrations';

/**
 * MigrationsDemo — the databases-sql topic's transactions demo (demo
 * block id `db:migrations`): the live begin/savepoint/rollback dance
 * narrated by the post count observed inside the transaction, beside
 * goose's migration ledger.
 */

type Status =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: MigrationsDemoData }
  | { state: 'error'; failure: Failure };

export function useMigrationsDemo(
  useCase: GetMigrationsDemoUseCase = container.resolve(DB_MIGRATIONS_DEMO_USE_CASE),
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

export function MigrationsDemo() {
  const { status, retry } = useMigrationsDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="migrations-loading" className="text-body-sm text-on-surface-variant">
        opening the transaction…
      </p>
    );
  }
  if (status.state === 'error') {
    return <ErrorPanel subject="the transactions demo" failure={status.failure} onRetry={retry} />;
  }

  const { history, steps, summary } = status.data;
  return (
    <div className="flex flex-col gap-md" data-testid="migrations-ready">
      <section data-testid="migrations-story" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">One transaction, narrated by its own count</h4>
        <ol className="flex flex-col gap-xs">
          {steps.map((step) => (
            <li key={step.step} className="flex flex-col gap-xxs rounded-md border border-outline px-sm py-xxs">
              <div className="flex flex-wrap items-baseline gap-xs">
                <span className="font-mono text-caption text-secondary">{step.step}.</span>
                <code className="font-mono text-body-sm text-on-surface">{step.sql}</code>
                <span data-testid="migrations-count" className="ml-auto rounded bg-surface-tint px-xs py-xxs font-mono text-caption text-secondary">
                  count = {step.postsVisible}
                </span>
              </div>
              <p className="text-caption text-on-surface-variant">{step.note}</p>
            </li>
          ))}
        </ol>
      </section>

      <section data-testid="migrations-ledger" className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">goose ledger (goose_db_version)</h4>
        <div className="flex flex-wrap gap-xxs">
          {history.map((row) => (
            <span key={`${row.version}-${row.appliedAt}`} className="rounded border border-outline px-xs py-xxs font-mono text-caption text-on-surface-variant">
              v{row.version} {row.applied ? '✓ applied' : '✗'} — {row.appliedAt}
            </span>
          ))}
        </div>
      </section>

      <p className="text-body-sm text-on-surface-variant">{summary}</p>
    </div>
  );
}

export default MigrationsDemo;
