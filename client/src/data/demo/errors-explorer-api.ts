import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { errorsExplorerDtoFromJson, type ErrorsExplorerDemoDto } from './errors-explorer-dto';

/**
 * ErrorsExplorerApi — the transport for GET /api/demo/errors/explorer,
 * in the same style as data/demo/jobs-console-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/errors/explorer';

export class ErrorsExplorerApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/errors/explorer — the table, panels, and live walk. */
  async get(): Promise<ErrorsExplorerDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, errorsExplorerDtoFromJson);
    });
  }
}
