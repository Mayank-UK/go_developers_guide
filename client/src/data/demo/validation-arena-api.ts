import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { validationArenaDtoFromJson, type ValidationArenaDemoDto } from './validation-arena-dto';

/**
 * ValidationArenaApi — the transport for GET /api/demo/validation/arena,
 * in the same style as data/content/content-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO. Converting typed exceptions into
 * Failed Results is the repository adapter's job.
 */

const DEMO_PATH = '/demo/validation/arena';

export class ValidationArenaApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/validation/arena — the judged candidate roster. */
  async get(): Promise<ValidationArenaDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, validationArenaDtoFromJson);
    });
  }
}
