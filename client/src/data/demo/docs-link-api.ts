import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { docsLinkDtoFromJson, type DocsLinkDto } from './docs-link-dto';

/**
 * DocsLinkApi — the transport for GET /api/demo/api/docs-link, in the
 * same style as data/content/content-api.ts: the request goes through the
 * shared guarded fetch pipeline (network down / non-JSON / non-2xx error
 * envelope all become typed AppExceptions), then the JSON payload is
 * validated into a DTO. Converting typed exceptions into Failed Results
 * is the repository adapter's job.
 */

const DEMO_PATH = '/demo/api/docs-link';

export class DocsLinkApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/api/docs-link — the spec facts card. */
  async get(): Promise<DocsLinkDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, docsLinkDtoFromJson);
    });
  }
}
