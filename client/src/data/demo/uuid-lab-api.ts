import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { uuidLabDtoFromJson, type UUIDLabDemoDto } from './uuid-lab-dto';

/**
 * UUIDLabApi — the transport for GET /api/demo/db/uuid, in the same
 * style as data/content/content-api.ts: the guarded fetch pipeline turns
 * every transport failure into a typed AppException, then the JSON is
 * validated into a DTO. Every call mints a FRESH batch server-side —
 * properties stable, values new.
 */

const DEMO_PATH = '/demo/db/uuid';

export class UUIDLabApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/uuid — a freshly minted UUIDv7 batch with
   * its measured properties. */
  async get(): Promise<UUIDLabDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, uuidLabDtoFromJson);
    });
  }
}
