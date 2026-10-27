import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { envInspectorDtoFromJson, type EnvInspectorDemoDto } from './env-inspector-dto';

/**
 * EnvInspectorApi — the transport for GET /api/demo/env/inspector, in
 * the same style as data/demo/debug-stall-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions), then the
 * JSON payload is validated into a DTO. The payload is derived, never
 * typed by hand — and the server redacts credential-bearing slots
 * before the payload exists, so no code path here can carry a secret.
 */

const DEMO_PATH = '/demo/env/inspector';

export class EnvInspectorApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/env/inspector — the resolved config, derived live. */
  async get(): Promise<EnvInspectorDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, envInspectorDtoFromJson);
    });
  }
}
