import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { securityStoryDtoFromJson, type SecurityStoryDemoDto } from './security-story-dto';

/**
 * SecurityStoryApi — the transport for the security topic's story
 * endpoints (GET /api/demo/security/{lab,pii}). One class, one instance
 * per demo path: the two payloads share one shape, so the adapter is
 * parameterized by the path the container hands it. The request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO. Both routes are themselves wrapped
 * in the topic's rate limiter server-side — hammering the demo from the
 * UI degrades to the typed 429 error panel, which is the honest
 * behavior the topic teaches.
 */
export class SecurityStoryApi {
  private readonly config: AppConfig;
  private readonly path: string;

  constructor(config: AppConfig, path: string) {
    this.config = config;
    this.path = path;
  }

  /** GET {apiBaseUrl}<path> — this demo's beats, proofs, and reasoning. */
  async get(): Promise<SecurityStoryDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, this.path);
      return parseDto(body, status, securityStoryDtoFromJson);
    });
  }
}
