import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { deployStackDtoFromJson, type DeployStackDemoDto } from './deploy-stack-dto';

/**
 * DeployStackApi — the transport for GET /api/demo/deploy/stack, in
 * the same style as data/demo/env-inspector-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions — including
 * the honest 503 the server answers with when the repo does not sit
 * beside the process), then the JSON payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/deploy/stack';

export class DeployStackApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/deploy/stack — the artifacts, parsed live. */
  async get(): Promise<DeployStackDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, deployStackDtoFromJson);
    });
  }
}
