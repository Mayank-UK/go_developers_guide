import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { i18nStoryDtoFromJson, type I18nStoryDemoDto } from './i18n-story-dto';

/**
 * I18nStoryApi — the transport for the i18n topic's negotiator story
 * endpoint (GET /api/demo/i18n/negotiator). The request goes through
 * the shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON payload
 * is validated into a DTO. No headers are set on purpose: the browser
 * attaches ITS OWN Accept-Language to the fetch automatically, and that
 * header is the demo's entire input — the server parses, negotiates,
 * and renders the seven beats from exactly what this request sent.
 */
export class I18nStoryApi {
  private readonly config: AppConfig;
  private readonly path: string;

  constructor(config: AppConfig, path: string) {
    this.config = config;
    this.path = path;
  }

  /** GET {apiBaseUrl}<path> — the story's beats, proofs, and reasoning. */
  async get(): Promise<I18nStoryDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, this.path);
      return parseDto(body, status, i18nStoryDtoFromJson);
    });
  }
}
