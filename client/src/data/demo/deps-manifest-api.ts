import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { depsManifestDtoFromJson, type DepsManifestDemoDto } from './deps-manifest-dto';

/**
 * DepsManifestApi — the transport for GET /api/demo/deps/manifest,
 * in the same style as data/demo/errors-explorer-api.ts: the request
 * goes through the shared guarded fetch pipeline (network down /
 * non-JSON / non-2xx error envelope all become typed AppExceptions),
 * then the JSON payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/deps/manifest';

export class DepsManifestApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/deps/manifest — manifest, build info, panels. */
  async get(): Promise<DepsManifestDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, depsManifestDtoFromJson);
    });
  }
}
