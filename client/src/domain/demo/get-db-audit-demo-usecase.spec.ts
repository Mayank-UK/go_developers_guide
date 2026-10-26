import { describe, expect, it } from 'vitest';
import { failed, success } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import { GetAuditDemoUseCase } from './get-db-audit-demo-usecase';
import type { AuditRepository } from './db-audit-repository';
import type { AuditDemoData } from './db-audit';

const fixture: AuditDemoData = {
  steps: [],
  committedTogether: true,
  rolledBackTogether: true,
  summary: 'same transaction, both writes',
};

describe('GetAuditDemoUseCase', () => {
  it('returns the repository payload on success', async () => {
    const useCase = new GetAuditDemoUseCase({
      get: () => Promise.resolve(success(fixture)),
    } as AuditRepository);
    const result = await useCase.execute();
    expect(result.ok && result.value.committedTogether).toBe(true);
  });

  it('passes failures through untouched', async () => {
    const useCase = new GetAuditDemoUseCase({
      get: () => Promise.resolve(failed(new NetworkFailure('down'))),
    } as AuditRepository);
    expect((await useCase.execute()).ok).toBe(false);
  });
});
