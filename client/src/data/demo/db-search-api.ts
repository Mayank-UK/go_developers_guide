import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { searchDtoFromJson, type SearchDemoDto } from './db-search-dto';

/** SearchApi — the transport for GET /api/demo/db/search. */

const DEMO_PATH = '/demo/db/search';

export class SearchApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/search — the ranked FTS match + plan. */
  async get(): Promise<SearchDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, searchDtoFromJson);
    });
  }
}
