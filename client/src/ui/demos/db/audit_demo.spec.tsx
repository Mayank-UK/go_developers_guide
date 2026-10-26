import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuditDemo } from './audit_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { NetworkFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetAuditDemoUseCase,
  DB_AUDIT_DEMO_USE_CASE,
} from '../../../domain/demo/get-db-audit-demo-usecase';
import type { AuditDemoData } from '../../../domain/demo/db-audit';

const fixture: AuditDemoData = {
  steps: [
    { phase: 'commit-open', action: 'BEGIN', liveTitle: '(no post yet)', auditRows: 0, note: 'the unit opens' },
    { phase: 'commit-change', action: 'two writes', liveTitle: 'audit scratch (edited)', auditRows: 2, note: 'visible inside only' },
    { phase: 'commit-done', action: 'COMMIT', liveTitle: 'audit scratch (edited)', auditRows: 2, note: 'true for everyone' },
    { phase: 'rollback-inside', action: 'two writes', liveTitle: 'audit scratch (doomed)', auditRows: 3, note: 'the doomed pair' },
    { phase: 'rollback-done', action: 'ROLLBACK', liveTitle: 'audit scratch (edited)', auditRows: 2, note: 'the log cannot tell a story that did not happen' },
  ],
  committedTogether: true,
  rolledBackTogether: true,
  summary: 'Same transaction, both writes.',
};

function stubUseCase(execute: () => Promise<Result<AuditDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(DB_AUDIT_DEMO_USE_CASE, { execute } as unknown as GetAuditDemoUseCase);
}

describe('AuditDemo', () => {
  it('renders both verdicts and the step story with its counts', async () => {
    stubUseCase(async () => success(fixture));
    render(<AuditDemo />);
    await waitFor(() => expect(screen.getByTestId('audit-ready')).toBeInTheDocument());
    expect(screen.getByTestId('audit-verdicts')).toHaveTextContent('committed together');
    expect(screen.getByTestId('audit-verdicts')).toHaveTextContent('rolled back together');
    const counts = screen.getAllByTestId('audit-count').map((node) => node.textContent);
    expect(counts).toEqual(['audit rows = 0', 'audit rows = 2', 'audit rows = 2', 'audit rows = 3', 'audit rows = 2']);
    expect(screen.getByTestId('audit-steps')).toHaveTextContent('audit scratch (doomed)');
  });

  it('shows the failing badges honestly when a proof does not hold', async () => {
    stubUseCase(async () => success({ ...fixture, committedTogether: false }));
    render(<AuditDemo />);
    await waitFor(() => expect(screen.getByTestId('audit-verdicts')).toBeInTheDocument());
    expect(screen.getByTestId('audit-verdicts')).toHaveTextContent('✗ commit proof failed');
  });

  it('degrades to the error panel on transport failure', async () => {
    stubUseCase(async () => failed(new NetworkFailure('Could not reach the server.')));
    render(<AuditDemo />);
    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');
  });
});
