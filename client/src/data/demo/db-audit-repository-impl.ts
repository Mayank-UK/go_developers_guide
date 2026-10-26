import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { AuditRepository } from '../../domain/demo/db-audit-repository';
import type { AuditDemoData } from '../../domain/demo/db-audit';
import { toAuditDemo } from './db-audit-dto';
import type { AuditApi } from './db-audit-api';

/** AuditRepositoryImpl — adapts the demo API to the domain port. */
export class AuditRepositoryImpl implements AuditRepository {
  private readonly api: AuditApi;

  constructor(api: AuditApi) {
    this.api = api;
  }

  async get(): Promise<Result<AuditDemoData>> {
    return safeRepositoryCall(async () => toAuditDemo(await this.api.get()));
  }
}
