import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { scalingStatelessDtoFromJson, type ScalingStatelessDemoDto } from './scaling-stateless-dto';

/**
 * ScalingStatelessApi — the transport for
 * GET /api/demo/scaling/stateless, in the same style as
 * data/demo/reliability-breaker-api.ts: the request goes through the
 * shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/scaling/stateless';

export class ScalingStatelessApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/scaling/stateless — the measured round. */
  async get(): Promise<ScalingStatelessDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, scalingStatelessDtoFromJson);
    });
  }
}
