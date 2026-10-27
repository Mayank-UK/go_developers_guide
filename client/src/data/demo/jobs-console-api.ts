import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { jobsConsoleDtoFromJson, type JobsConsoleDemoDto } from './jobs-console-dto';

/**
 * JobsConsoleApi — the transport for GET /api/demo/jobs/console, in the
 * same style as data/demo/messaging-stream-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions — the server's
 * honest 503 for an unreachable Redis or a not-running worker
 * included), then the JSON payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/jobs/console';

export class JobsConsoleApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/jobs/console — the story's ladders, beats, and proofs. */
  async get(): Promise<JobsConsoleDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, jobsConsoleDtoFromJson);
    });
  }
}
