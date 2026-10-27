import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { obsViewerDtoFromJson, type ObsViewerDemoDto } from './obs-viewer-dto';

/**
 * ObsViewerApi — the transport for GET /api/demo/obs/viewer, in the
 * same style as data/demo/arch-tracer-api.ts: the request goes through
 * the shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/obs/viewer';

export class ObsViewerApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/obs/viewer — the dissected line + panels. */
  async get(): Promise<ObsViewerDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, obsViewerDtoFromJson);
    });
  }
}
