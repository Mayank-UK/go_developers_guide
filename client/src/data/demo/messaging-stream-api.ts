import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { messagingStreamDtoFromJson, type MessagingStreamDemoDto } from './messaging-stream-dto';

/**
 * MessagingStreamApi — the transport for GET /api/demo/msg/stream, in
 * the same style as data/demo/redis-playground-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions — the server's
 * honest 503 for an unreachable Redis included), then the JSON payload
 * is validated into a DTO.
 */

const DEMO_PATH = '/demo/msg/stream';

export class MessagingStreamApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/msg/stream — the story's beats and proofs. */
  async get(): Promise<MessagingStreamDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, messagingStreamDtoFromJson);
    });
  }
}
