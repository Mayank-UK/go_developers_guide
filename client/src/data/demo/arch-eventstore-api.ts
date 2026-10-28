import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { archEventstoreDtoFromJson, type ArchEventstoreDemoDto } from './arch-eventstore-dto';

/**
 * ArchEventstoreApi — the transport for
 * GET /api/demo/arch/eventstore, in the same style as
 * data/demo/thirdparty-hub-api.ts: the request goes through the shared
 * guarded fetch pipeline (network down / non-JSON / non-2xx error
 * envelope all become typed AppExceptions), then the JSON payload is
 * validated into a DTO.
 */

const DEMO_PATH = '/demo/arch/eventstore';

export class ArchEventstoreApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/arch/eventstore — the event-store exhibit. */
  async get(): Promise<ArchEventstoreDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, archEventstoreDtoFromJson);
    });
  }
}
