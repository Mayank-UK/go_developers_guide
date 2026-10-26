import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { middlewarePipelineDtoFromJson, type MiddlewarePipelineDemoDto } from './middleware-pipeline-dto';

/**
 * MiddlewarePipelineApi — the transport for
 * GET /api/demo/middleware/pipeline, in the same style as
 * data/content/content-api.ts: the request goes through the shared
 * guarded fetch pipeline (network down / non-JSON / non-2xx error
 * envelope all become typed AppExceptions), then the JSON payload is
 * validated into a DTO. Converting typed exceptions into Failed Results
 * is the repository adapter's job.
 */

const DEMO_PATH = '/demo/middleware/pipeline';

export class MiddlewarePipelineApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/middleware/pipeline — the traced exhibit-chain run. */
  async get(): Promise<MiddlewarePipelineDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, middlewarePipelineDtoFromJson);
    });
  }
}
