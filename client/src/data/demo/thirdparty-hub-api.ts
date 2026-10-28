import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { thirdPartyHubDtoFromJson, type ThirdPartyHubDemoDto } from './thirdparty-hub-dto';

/**
 * ThirdPartyHubApi — the transport for
 * GET /api/demo/thirdparty/hub, in the same style as
 * data/demo/best-checklist-api.ts: the request goes through the shared
 * guarded fetch pipeline (network down / non-JSON / non-2xx error
 * envelope all become typed AppExceptions), then the JSON payload is
 * validated into a DTO.
 */

const DEMO_PATH = '/demo/thirdparty/hub';

export class ThirdPartyHubApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/thirdparty/hub — the adapter-surface hub. */
  async get(): Promise<ThirdPartyHubDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, thirdPartyHubDtoFromJson);
    });
  }
}
