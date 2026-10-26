import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { reqresInspectorDtoFromJson, type ReqresInspectorDemoDto } from './reqres-inspector-dto';

/**
 * ReqresInspectorApi — the transport for GET /api/demo/reqres/inspector,
 * in the same style as data/content/content-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO. Converting typed exceptions into
 * Failed Results is the repository adapter's job.
 */

const DEMO_PATH = '/demo/reqres/inspector';

export class ReqresInspectorApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/reqres/inspector — this very request, judged. */
  async get(): Promise<ReqresInspectorDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, reqresInspectorDtoFromJson);
    });
  }
}
