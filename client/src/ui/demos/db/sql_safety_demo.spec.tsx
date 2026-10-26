import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SQLSafetyDemo } from './sql_safety_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { NetworkFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetSQLSafetyDemoUseCase,
  DB_SQL_SAFETY_DEMO_USE_CASE,
} from '../../../domain/demo/get-sql-safety-demo-usecase';
import type { SQLSafetyDemoData } from '../../../domain/demo/sql-safety';

const fixture: SQLSafetyDemoData = {
  attacks: [
    {
      input: "x' OR '1'='1",
      intent: 'match every row',
      parameterizedSql: 'SELECT count(*) FROM posts WHERE title = $1',
      rowsMatched: 0,
      postsAfter: 6,
      assembledSql: "SELECT count(*) FROM posts WHERE title = 'x' OR '1'='1'",
      assembledEffect: 'every post leaks',
      verdict: 'inert — bound as data, compared as data',
    },
    {
      input: "'; DROP TABLE posts; --",
      intent: 'run a second statement',
      parameterizedSql: 'SELECT count(*) FROM posts WHERE title = $1',
      rowsMatched: 0,
      postsAfter: 6,
      assembledSql: 'SELECT count(*) FROM posts … DROP TABLE posts; --',
      assembledEffect: 'the table would drop',
      verdict: 'inert — bound as data, compared as data',
    },
  ],
  summary: 'SQL text and values travel as separate messages.',
};

function stubUseCase(execute: () => Promise<Result<SQLSafetyDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(DB_SQL_SAFETY_DEMO_USE_CASE, { execute } as unknown as GetSQLSafetyDemoUseCase);
}

describe('SQLSafetyDemo', () => {
  it('renders both attacks with their live outcomes and exhibits', async () => {
    stubUseCase(async () => success(fixture));
    render(<SQLSafetyDemo />);
    await waitFor(() => expect(screen.getByTestId('sql-safety-ready')).toBeInTheDocument());
    const attacks = screen.getAllByTestId('sql-safety-attack');
    expect(attacks).toHaveLength(2);
    expect(attacks[0]).toHaveTextContent('$1');
    expect(attacks[0]).toHaveTextContent('0 rows matched');
    expect(attacks[0]).toHaveTextContent('6 posts standing');
    expect(attacks[0]).toHaveTextContent("OR '1'='1");
    expect(attacks[1]).toHaveTextContent('DROP TABLE posts');
    const verdicts = screen.getAllByTestId('sql-safety-verdict');
    expect(verdicts[0]).toHaveTextContent('inert');
  });

  it('degrades to the error panel on transport failure', async () => {
    stubUseCase(async () => failed(new NetworkFailure('Could not reach the server.')));
    render(<SQLSafetyDemo />);
    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');
  });
});
