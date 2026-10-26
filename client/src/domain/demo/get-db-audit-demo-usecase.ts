import type { Result } from '../../core/result';
import type { AuditRepository } from './db-audit-repository';
import type { AuditDemoData } from './db-audit';

/** DB_AUDIT_DEMO_USE_CASE — the container token for this demo's use case. */
export const DB_AUDIT_DEMO_USE_CASE = Symbol('GetAuditDemoUseCase');

/** GetAuditDemoUseCase — loads the audit-in-transaction payload. */
export class GetAuditDemoUseCase {
  private readonly repository: AuditRepository;

  constructor(repository: AuditRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<AuditDemoData>> {
    return this.repository.get();
  }
}
