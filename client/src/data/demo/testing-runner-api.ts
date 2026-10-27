import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { testingRunnerDtoFromJson, type TestingRunnerDemoDto } from './testing-runner-dto';

/**
 * TestingRunnerApi — the transport for GET /api/demo/testing/runner, in
 * the same style as data/demo/obs-trace-api.ts: the request goes through
 * the shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON payload
 * is validated into a DTO. A toolchain hiccup on the server arrives as
 * the typed 503 envelope and guards into a ServerFailure here.
 */

const DEMO_PATH = '/demo/testing/runner';

export class TestingRunnerApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/testing/runner — the parsed timeline + shape. */
  async get(): Promise<TestingRunnerDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, testingRunnerDtoFromJson);
    });
  }
}
