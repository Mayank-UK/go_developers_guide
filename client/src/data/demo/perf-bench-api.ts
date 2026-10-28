import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { perfBenchDtoFromJson, type PerfBenchDemoDto } from './perf-bench-dto';

/**
 * PerfBenchApi — the transport for GET /api/demo/perf/bench, in the
 * same style as data/demo/deploy-stack-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions — including
 * the honest 503 the server answers with when the toolchain or source
 * tree is absent), then the JSON payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/perf/bench';

export class PerfBenchApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/perf/bench — the boot-cached run, parsed. */
  async get(): Promise<PerfBenchDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, perfBenchDtoFromJson);
    });
  }
}
