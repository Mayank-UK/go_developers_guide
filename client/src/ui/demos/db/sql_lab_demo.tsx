import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetSQLLabDemoUseCase,
  DB_SQL_LAB_DEMO_USE_CASE,
} from '../../../domain/demo/get-sql-lab-demo-usecase';
import type { SQLLabDemoData } from '../../../domain/demo/sql-lab';

/**
 * SQLLabDemo — the databases-sql topic's SQL-by-hand demo (demo block id
 * `db:sql-lab`): the CTE and window-function exhibits, each with the SQL
 * that ran, what it teaches, and the rows it returned.
 */

type Status =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: SQLLabDemoData }
  | { state: 'error'; failure: Failure };

export function useSQLLabDemo(
  useCase: GetSQLLabDemoUseCase = container.resolve(DB_SQL_LAB_DEMO_USE_CASE),
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

export function SQLLabDemo() {
  const { status, retry } = useSQLLabDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="sql-lab-loading" className="text-body-sm text-on-surface-variant">
        running the exhibits…
      </p>
    );
  }
  if (status.state === 'error') {
    return <ErrorPanel subject="the SQL lab" failure={status.failure} onRetry={retry} />;
  }

  return (
    <div className="flex flex-col gap-md" data-testid="sql-lab-ready">
      {status.data.queries.map((query) => (
        <section key={query.id} data-testid={`sql-lab-${query.id}`} className="flex flex-col gap-xs rounded-md border border-outline px-md py-sm">
          <h4 className="font-display text-title-sm text-on-surface">{query.title}</h4>
          <p className="text-body-sm text-on-surface-variant">{query.teach}</p>
          <pre className="overflow-x-auto rounded bg-surface-tint px-sm py-xxs font-mono text-caption text-on-surface">
            <code>{query.sql}</code>
          </pre>
          <div className="overflow-x-auto">
            <table className="w-full text-left font-mono text-caption">
              <thead className="text-on-surface-variant">
                <tr>
                  {query.columns.map((column) => (
                    <th key={column} scope="col" className="pr-sm py-xxs">{column}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {query.rows.map((row, index) => (
                  <tr key={index} className="border-t border-outline">
                    {row.map((cell, cellIndex) => (
                      <td key={cellIndex} className="pr-sm py-xxs text-on-surface">{cell}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ))}
    </div>
  );
}

export default SQLLabDemo;
