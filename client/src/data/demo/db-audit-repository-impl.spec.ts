import { describe, expect, it } from 'vitest';
import { AuditRepositoryImpl } from './db-audit-repository-impl';
import type { AuditApi } from './db-audit-api';
import type { AuditDemoData } from '../../domain/demo/db-audit';

const fixture = {
  steps: [],
  committedTogether: true,
  rolledBackTogether: true,
  summary: 's',
} as AuditDemoData;

describe('AuditRepositoryImpl', () => {
  it('wraps the api payload in a success Result', async () => {
    const repo = new AuditRepositoryImpl({ get: async () => fixture } as unknown as AuditApi);
    expect((await repo.get()).ok).toBe(true);
  });

  it('guards api rejections into failed Results', async () => {
    const repo = new AuditRepositoryImpl({
      get: async () => { throw new Error('boom'); },
    } as unknown as AuditApi);
    expect((await repo.get()).ok).toBe(false);
  });
});
