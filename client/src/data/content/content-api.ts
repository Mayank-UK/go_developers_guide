import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import {
  contentIndexDtoFromJson,
  topicDetailDtoFromJson,
  type ContentIndexDto,
  type TopicDetailDto,
} from './content-dto';

/**
 * ContentApi — the transport for the content endpoints, in the same style as
 * data/health/health-api.ts: resolve the request against the configured API
 * base URL, send it through the shared guarded fetch pipeline (network down /
 * non-JSON / non-2xx error envelope all become typed AppExceptions — see
 * core/error-handling/guards.ts), then validate the JSON payload into a DTO.
 * Converting the typed exceptions into Failed Results is the repository
 * adapter's job, so domain code never sees a thrown transport error.
 */

const CONTENT_PATH = '/content';
const topicDetailPath = (topicId: string): string =>
  `/content/${encodeURIComponent(topicId)}`;

export class ContentApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/content — the curriculum index (groups + summaries). */
  async list(): Promise<ContentIndexDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, CONTENT_PATH);
      return parseDto(body, status, contentIndexDtoFromJson);
    });
  }

  /** GET {apiBaseUrl}/content/{topicId} — one topic's sections + blocks. */
  async byId(topicId: string): Promise<TopicDetailDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, topicDetailPath(topicId));
      return parseDto(body, status, topicDetailDtoFromJson);
    });
  }
}
