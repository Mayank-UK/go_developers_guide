import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { postsWorkbenchDtoFromJson, type PostsWorkbenchDemoDto } from './posts-workbench-dto';

/**
 * PostsWorkbenchApi — the transport for GET /api/demo/api/posts-workbench
 * (the api-design topic's contract card), in the same style as the other
 * demo APIs: the request goes through the shared guarded fetch pipeline,
 * then the JSON payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/api/posts-workbench';

export class PostsWorkbenchApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/api/posts-workbench — the route table + rules. */
  async get(): Promise<PostsWorkbenchDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, postsWorkbenchDtoFromJson);
    });
  }
}
