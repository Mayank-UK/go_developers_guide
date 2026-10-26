import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { auditDtoFromJson, type AuditDemoDto } from './db-audit-dto';

/** AuditApi — the transport for GET /api/demo/db/audit. */

const DEMO_PATH = '/demo/db/audit';

export class AuditApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/audit — the in-transaction audit story. */
  async get(): Promise<AuditDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, auditDtoFromJson);
    });
  }
}
