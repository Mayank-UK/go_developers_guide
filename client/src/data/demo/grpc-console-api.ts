import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { grpcConsoleDtoFromJson, type GRPCConsoleDemoDto } from './grpc-console-dto';

/**
 * GRPCConsoleApi — the transport for GET /api/demo/api/grpc-console, in
 * the same style as data/content/content-api.ts: the request goes through
 * the shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON payload
 * is validated into a DTO. Converting typed exceptions into Failed
 * Results is the repository adapter's job.
 */

const DEMO_PATH = '/demo/api/grpc-console';

export class GRPCConsoleApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/api/grpc-console — the six exhibit reports. */
  async get(): Promise<GRPCConsoleDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, grpcConsoleDtoFromJson);
    });
  }
}
