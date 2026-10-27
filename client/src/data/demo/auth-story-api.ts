import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { authStoryDtoFromJson, type AuthStoryDemoDto } from './auth-story-dto';

/**
 * AuthStoryApi — the transport for the authentication topic's story
 * endpoints (GET /api/demo/auth/{sessions,api-keys,compare,jwt,oauth,
 * hashing,refresh}). One class, one instance per demo path: the seven
 * payloads share one shape, so the adapter is parameterized by the
 * path the container hands it. The request goes through the shared
 * guarded fetch pipeline (network down / non-JSON / non-2xx error
 * envelope all become typed AppExceptions), then the JSON payload is
 * validated into a DTO.
 */
export class AuthStoryApi {
  private readonly config: AppConfig;
  private readonly path: string;

  constructor(config: AppConfig, path: string) {
    this.config = config;
    this.path = path;
  }

  /** GET {apiBaseUrl}<path> — this demo's beats, proofs, and reasoning. */
  async get(): Promise<AuthStoryDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, this.path);
      return parseDto(body, status, authStoryDtoFromJson);
    });
  }
}
