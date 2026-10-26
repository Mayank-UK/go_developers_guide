import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MigrationsDemo } from './migrations_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { NetworkFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetMigrationsDemoUseCase,
  DB_MIGRATIONS_DEMO_USE_CASE,
} from '../../../domain/demo/get-db-migrations-demo-usecase';
import type { MigrationsDemoData } from '../../../domain/demo/db-migrations';

const fixture: MigrationsDemoData = {
  history: [
    { version: 0, applied: true, appliedAt: '2026-10-26 21:20:00' },
    { version: 1, applied: true, appliedAt: '2026-10-26 21:20:00' },
  ],
  steps: [
    { step: 1, action: 'begin', sql: 'BEGIN', postsVisible: 6, note: 'a unit opens' },
    { step: 2, action: 'insert scratch post', sql: 'INSERT …', postsVisible: 7, note: 'visible inside only' },
    { step: 4, action: 'insert second post', sql: 'INSERT …', postsVisible: 8, note: 'still uncommitted' },
    { step: 5, action: 'rollback to savepoint', sql: 'ROLLBACK TO SAVEPOINT sp1', postsVisible: 7, note: 'partial unwind' },
    { step: 6, action: 'rollback', sql: 'ROLLBACK', postsVisible: 6, note: 'never happened' },
  ],
  summary: 'Atomicity is the boundary\'s gift.',
};

function stubUseCase(execute: () => Promise<Result<MigrationsDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(DB_MIGRATIONS_DEMO_USE_CASE, { execute } as unknown as GetMigrationsDemoUseCase);
}

describe('MigrationsDemo', () => {
  it('renders the story steps with their counts and the ledger', async () => {
    stubUseCase(async () => success(fixture));
    render(<MigrationsDemo />);
    await waitFor(() => expect(screen.getByTestId('migrations-ready')).toBeInTheDocument());
    const counts = screen.getAllByTestId('migrations-count').map((node) => node.textContent);
    expect(counts).toEqual(['count = 6', 'count = 7', 'count = 8', 'count = 7', 'count = 6']);
    expect(screen.getByTestId('migrations-story')).toHaveTextContent('ROLLBACK TO SAVEPOINT sp1');
    expect(screen.getByTestId('migrations-ledger')).toHaveTextContent('v0 ✓ applied');
    expect(screen.getByTestId('migrations-ledger')).toHaveTextContent('v1');
  });

  it('degrades to the error panel on transport failure', async () => {
    stubUseCase(async () => failed(new NetworkFailure('Could not reach the server.')));
    render(<MigrationsDemo />);
    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');
  });
});
