import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetSQLSafetyDemoUseCase,
  DB_SQL_SAFETY_DEMO_USE_CASE,
} from '../../../domain/demo/get-sql-safety-demo-usecase';
import type { SQLSafetyDemoData } from '../../../domain/demo/sql-safety';

/**
 * SQLSafetyDemo — the databases-sql topic's injection demo (demo block id
 * `db:sql-safety`). Renders the canonical attacks two ways at once: the
 * live parameterized outcome (rows matched, posts after) against the
 * never-executed concatenation exhibit — the museum piece beside the
 * proof.
 */

type Status =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: SQLSafetyDemoData }
  | { state: 'error'; failure: Failure };

export function useSQLSafetyDemo(
  useCase: GetSQLSafetyDemoUseCase = container.resolve(DB_SQL_SAFETY_DEMO_USE_CASE),
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

export function SQLSafetyDemo() {
  const { status, retry } = useSQLSafetyDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="sql-safety-loading" className="text-body-sm text-on-surface-variant">
        sending the attacks as $1…
      </p>
    );
  }
  if (status.state === 'error') {
    return <ErrorPanel subject="the sql-safety demo" failure={status.failure} onRetry={retry} />;
  }

  const { attacks, summary } = status.data;
  return (
    <div className="flex flex-col gap-md" data-testid="sql-safety-ready">
      {attacks.map((attack) => (
        <section key={attack.input} data-testid="sql-safety-attack" className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
          <div className="flex flex-wrap items-baseline justify-between gap-xs">
            <code className="rounded bg-surface px-xs py-xxs font-mono text-body-sm text-error">{attack.input}</code>
            <span data-testid="sql-safety-verdict" className="text-caption text-secondary">{attack.verdict}</span>
          </div>
          <p className="text-caption text-on-surface-variant">intent: {attack.intent}</p>
          <div className="flex flex-col gap-xxs font-mono text-caption">
            <p className="text-on-surface">
              <span className="text-secondary">executed:</span> {attack.parameterizedSql}
            </p>
            <p className="text-on-surface-variant">
              → {attack.rowsMatched} rows matched · {attack.postsAfter} posts standing
            </p>
            <p className="text-error">
              <span className="text-secondary">never executed:</span> {attack.assembledSql}
            </p>
            <p className="text-error text-caption">→ {attack.assembledEffect}</p>
          </div>
        </section>
      ))}
      <p className="text-body-sm text-on-surface-variant">{summary}</p>
    </div>
  );
}

export default SQLSafetyDemo;
