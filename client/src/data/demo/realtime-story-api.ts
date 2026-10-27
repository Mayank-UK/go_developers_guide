import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { realtimeStoryDtoFromJson, type RealtimeStoryDemoDto } from './realtime-story-dto';

/**
 * RealtimeStoryApi — the transport for the realtime topic's story
 * endpoints (GET /api/demo/realtime/{ticker,chat}). One class, one
 * instance per demo path: the two payloads share one shape, so the
 * adapter is parameterized by the path the container hands it. The
 * request goes through the shared guarded fetch pipeline (network down /
 * non-JSON / non-2xx error envelope all become typed AppExceptions),
 * then the JSON payload is validated into a DTO. The LIVE transports
 * (SSE stream, WS room) are separate endpoints the demo panels hit
 * with browser-native EventSource/WebSocket — a JSON GET cannot carry
 * a stream.
 */
export class RealtimeStoryApi {
  private readonly config: AppConfig;
  private readonly path: string;

  constructor(config: AppConfig, path: string) {
    this.config = config;
    this.path = path;
  }

  /** GET {apiBaseUrl}<path> — this demo's beats, proofs, and reasoning. */
  async get(): Promise<RealtimeStoryDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, this.path);
      return parseDto(body, status, realtimeStoryDtoFromJson);
    });
  }
}
