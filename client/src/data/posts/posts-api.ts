import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, guardedSendJson, parseDto } from '../../core/error-handling/guards';
import { postInputToJson, postFromJson, postPageFromJson, postsListQueryString } from './posts-dto';
import type { Post, PostInput, PostPage, PostsListQuery } from '../../domain/posts/post';

/**
 * PostsApi — the transport for the /api/posts resource, in the same style
 * as data/content/content-api.ts: every call rides the shared guarded
 * fetch pipeline (network down / non-JSON / non-2xx error envelope all
 * become typed AppExceptions), then 2xx bodies are validated into DTOs.
 * A 422 issue envelope or a 404 miss therefore reaches the repository as
 * a typed ServerException carrying the envelope's code, message, and
 * requestId — which is exactly what the workbench renders honestly.
 */

const POSTS_PATH = '/posts';

export class PostsApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/posts — one page of the collection. */
  async list(query: PostsListQuery): Promise<PostPage> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(
        this.config,
        `${POSTS_PATH}${postsListQueryString(query)}`,
      );
      return parseDto(body, status, postPageFromJson);
    });
  }

  /** POST {apiBaseUrl}/posts — create; the stored post returns (201). */
  async create(input: PostInput): Promise<Post> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedSendJson(this.config, POSTS_PATH, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: postInputToJson(input),
      });
      return parseDto(body, status, postFromJson);
    });
  }

  /** PUT {apiBaseUrl}/posts/{id} — full replacement (200). */
  async replace(id: string, input: PostInput): Promise<Post> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedSendJson(this.config, `${POSTS_PATH}/${id}`, {
        method: 'PUT',
        headers: { 'content-type': 'application/json' },
        body: postInputToJson(input),
      });
      return parseDto(body, status, postFromJson);
    });
  }

  /** DELETE {apiBaseUrl}/posts/{id} — 204; a miss throws the 404 envelope. */
  async remove(id: string): Promise<void> {
    await guardedApiCall(async () => {
      await guardedSendJson(this.config, `${POSTS_PATH}/${id}`, { method: 'DELETE' });
    });
  }
}
