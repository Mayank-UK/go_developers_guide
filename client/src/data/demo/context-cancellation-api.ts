import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import {
  contextCancellationDtoFromJson,
  type ContextCancellationDemoDto,
} from './context-cancellation-dto';

/**
 * ContextCancellationApi — the transport for
 * GET /api/demo/conc/context, in the same style as
 * data/demo/runtime-types-api.ts: the request goes through the shared
 * guarded fetch pipeline (network down / non-JSON / non-2xx error
 * envelope all become typed AppExceptions), then the JSON payload is
 * validated into a DTO. Converting typed exceptions into Failed Results
 * is the repository adapter's job.
 */

const DEMO_PATH = '/demo/conc/context';

export class ContextCancellationApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/conc/context — the cancellation choreography. */
  async get(): Promise<ContextCancellationDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, contextCancellationDtoFromJson);
    });
  }
}
