import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { bestChecklistDtoFromJson, type BestChecklistDemoDto } from './best-checklist-dto';

/**
 * BestChecklistApi — the transport for
 * GET /api/demo/best/checklist, in the same style as
 * data/demo/scaling-stateless-api.ts: the request goes through the
 * shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/best/checklist';

export class BestChecklistApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/best/checklist — the boot-cached self-audit. */
  async get(): Promise<BestChecklistDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, bestChecklistDtoFromJson);
    });
  }
}
