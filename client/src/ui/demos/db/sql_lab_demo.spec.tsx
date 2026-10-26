import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SQLLabDemo } from './sql_lab_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { NetworkFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetSQLLabDemoUseCase,
  DB_SQL_LAB_DEMO_USE_CASE,
} from '../../../domain/demo/get-sql-lab-demo-usecase';
import type { SQLLabDemoData } from '../../../domain/demo/sql-lab';

const fixture: SQLLabDemoData = {
  queries: [
    {
      id: 'tag-totals',
      title: 'A CTE chain',
      sql: 'WITH tagged AS (…)',
      teach: 'name the intermediate',
      columns: ['tag', 'posts'],
      rows: [['api', 2], ['go', 4]],
    },
    {
      id: 'publish-gaps',
      title: 'LAG over the timeline',
      sql: 'SELECT … LAG(…)',
      teach: 'reach back one row',
      columns: ['title', 'created', 'daysSincePrevious'],
      rows: [['Why Go for backends', '2026-01-05', '—'], ['ServeMux patterns in Go 1.22', '2026-01-07', 2]],
    },
  ],
};

function stubUseCase(execute: () => Promise<Result<SQLLabDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(DB_SQL_LAB_DEMO_USE_CASE, { execute } as unknown as GetSQLLabDemoUseCase);
}

describe('SQLLabDemo', () => {
  it('renders every exhibit with its SQL, teach line, and rows', async () => {
    stubUseCase(async () => success(fixture));
    render(<SQLLabDemo />);
    await waitFor(() => expect(screen.getByTestId('sql-lab-ready')).toBeInTheDocument());
    expect(screen.getByTestId('sql-lab-tag-totals')).toHaveTextContent('WITH tagged AS');
    expect(screen.getByTestId('sql-lab-tag-totals')).toHaveTextContent('name the intermediate');
    expect(screen.getByTestId('sql-lab-tag-totals')).toHaveTextContent('go');
    expect(screen.getByTestId('sql-lab-publish-gaps')).toHaveTextContent('—');
    expect(screen.getByTestId('sql-lab-publish-gaps')).toHaveTextContent('2026-01-07');
  });

  it('degrades to the error panel on transport failure', async () => {
    stubUseCase(async () => failed(new NetworkFailure('Could not reach the server.')));
    render(<SQLLabDemo />);
    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');
  });
});
