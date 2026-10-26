import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetSearchDemoUseCase,
  DB_SEARCH_DEMO_USE_CASE,
} from '../../../domain/demo/get-db-search-demo-usecase';
import type { SearchDemoData } from '../../../domain/demo/db-search';

/**
 * SearchDemo — the databases-sql topic's full-text search demo (demo
 * block id `db:search`): the fixed natural-language query, the tsquery it
 * stemmed into, the ranked matches, the EXPLAIN plan, and the GIN
 * index's own definition from the catalog.
 */

type Status =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: SearchDemoData }
  | { state: 'error'; failure: Failure };

export function useSearchDemo(
  useCase: GetSearchDemoUseCase = container.resolve(DB_SEARCH_DEMO_USE_CASE),
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

export function SearchDemo() {
  const { status, retry } = useSearchDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="search-loading" className="text-body-sm text-on-surface-variant">
        stemming and matching…
      </p>
    );
  }
  if (status.state === 'error') {
    return <ErrorPanel subject="the search demo" failure={status.failure} onRetry={retry} />;
  }

  const { query, tsQuery, sql, matches, plan, indexDefinition, teach } = status.data;
  return (
    <div className="flex flex-col gap-md" data-testid="search-ready">
      <section data-testid="search-query" className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <div className="flex flex-wrap items-baseline gap-xs">
          <span className="text-body-sm text-on-surface-variant">query:</span>
          <code className="rounded bg-surface px-xs py-xxs font-mono text-body-sm text-on-surface">{query}</code>
          <span className="text-body-sm text-on-surface-variant">→ stems to</span>
          <code className="font-mono text-body-sm text-secondary">{tsQuery}</code>
        </div>
        <p className="text-caption text-on-surface-variant">{teach}</p>
      </section>

      <section data-testid="search-matches" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">Ranked matches ({matches.length})</h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">title</th>
                <th scope="col" className="px-sm py-xxs">ts_rank</th>
              </tr>
            </thead>
            <tbody>
              {matches.map((match) => (
                <tr key={match.title} className="border-t border-outline">
                  <td className="px-sm py-xxs text-on-surface">{match.title}</td>
                  <td className="px-sm py-xxs text-secondary">{match.rank}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="search-plan" className="flex flex-col gap-xxs">
        <h4 className="font-display text-title-sm text-on-surface">The plan PostgreSQL chose</h4>
        <pre className="overflow-x-auto rounded bg-surface-tint px-sm py-xxs font-mono text-caption text-on-surface">
          <code>{plan.join('\n')}</code>
        </pre>
        <pre className="overflow-x-auto rounded border border-outline px-sm py-xxs font-mono text-caption text-on-surface-variant">
          <code>{indexDefinition}</code>
        </pre>
      </section>

      <section data-testid="search-sql">
        <pre className="overflow-x-auto rounded bg-surface-tint px-sm py-xxs font-mono text-caption text-on-surface">
          <code>{sql}</code>
        </pre>
      </section>
    </div>
  );
}

export default SearchDemo;
