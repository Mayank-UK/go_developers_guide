import type { Result } from '../../core/result';
import type { AuditDemoData } from './db-audit';

/** AuditRepository — the domain-facing port for the audit demo payload. */
export interface AuditRepository {
  /** The commit-together / roll-back-together story. */
  get(): Promise<Result<AuditDemoData>>;
}
