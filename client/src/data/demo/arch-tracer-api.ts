import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { archTracerDtoFromJson, type ArchTracerDemoDto } from './arch-tracer-dto';

/**
 * ArchTracerApi — the transport for GET /api/demo/arch/tracer, in the
 * same style as data/demo/deps-manifest-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/arch/tracer';

export class ArchTracerApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/arch/tracer — the traced timeline + panels. */
  async get(): Promise<ArchTracerDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, archTracerDtoFromJson);
    });
  }
}
