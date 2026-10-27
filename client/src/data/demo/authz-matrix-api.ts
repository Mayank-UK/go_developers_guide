import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { authzMatrixDtoFromJson, type AuthzMatrixDemoDto } from './authz-matrix-dto';

/**
 * AuthzMatrixApi — the transport for GET /api/demo/authz/matrix, in the
 * same style as data/content/content-api.ts: the request goes through the
 * shared guarded fetch pipeline (network down / non-JSON / non-2xx error
 * envelope all become typed AppExceptions), then the JSON payload is
 * validated into a DTO. The endpoint is deliberately unauthenticated —
 * the matrix is a story payload; the guarded proof routes it teaches
 * answer the 401-vs-403 boundary themselves.
 */

const DEMO_PATH = '/demo/authz/matrix';

export class AuthzMatrixApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/authz/matrix — the matrix's beats and proofs. */
  async get(): Promise<AuthzMatrixDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, authzMatrixDtoFromJson);
    });
  }
}
