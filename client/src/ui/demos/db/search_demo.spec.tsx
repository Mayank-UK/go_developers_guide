import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SearchDemo } from './search_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { NetworkFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetSearchDemoUseCase,
  DB_SEARCH_DEMO_USE_CASE,
} from '../../../domain/demo/get-db-search-demo-usecase';
import type { SearchDemoData } from '../../../domain/demo/db-search';

const fixture: SearchDemoData = {
  query: 'logging or errors',
  tsQuery: "'log' | 'error'",
  sql: 'SELECT title, ts_rank(…) …',
  matches: [
    { title: 'Errors are values', rank: '0.0414' },
    { title: 'Structured logging with slog', rank: '0.0380' },
    { title: 'Pagination that tells the truth', rank: '0.0304' },
  ],
  plan: ['Seq Scan on posts  (cost=0.00..1.06 rows=6 width=68)'],
  indexDefinition: 'CREATE INDEX posts_search_idx ON public.posts USING gin (to_tsvector(…))',
  teach: 'stem both sides, rank the survivors',
};

function stubUseCase(execute: () => Promise<Result<SearchDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(DB_SEARCH_DEMO_USE_CASE, { execute } as unknown as GetSearchDemoUseCase);
}

describe('SearchDemo', () => {
  it('renders the stemmed query, ranked matches, plan, and index', async () => {
    stubUseCase(async () => success(fixture));
    render(<SearchDemo />);
    await waitFor(() => expect(screen.getByTestId('search-ready')).toBeInTheDocument());
    expect(screen.getByTestId('search-query')).toHaveTextContent('logging or errors');
    expect(screen.getByTestId('search-query')).toHaveTextContent("'log' | 'error'");
    const matches = screen.getByTestId('search-matches');
    expect(matches).toHaveTextContent('Ranked matches (3)');
    expect(matches).toHaveTextContent('Errors are values');
    expect(matches).toHaveTextContent('0.0414');
    expect(screen.getByTestId('search-plan')).toHaveTextContent('Seq Scan');
    expect(screen.getByTestId('search-plan')).toHaveTextContent('USING gin');
  });

  it('degrades to the error panel on transport failure', async () => {
    stubUseCase(async () => failed(new NetworkFailure('Could not reach the server.')));
    render(<SearchDemo />);
    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');
  });
});
