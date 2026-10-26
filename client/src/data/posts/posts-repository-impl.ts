import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { PostsRepository } from '../../domain/posts/post-repository';
import type { Post, PostInput, PostPage, PostsListQuery } from '../../domain/posts/post';
import type { PostsApi } from './posts-api';

/**
 * PostsRepositoryImpl — adapts the posts API to the domain port. Every
 * thrown transport error (network, 404 envelope, 422 issue envelope,
 * malformed payload) is guarded by safeRepositoryCall into a Failed
 * Result, so the workbench never needs try/catch and no rejection goes
 * unhandled.
 */
export class PostsRepositoryImpl implements PostsRepository {
  private readonly api: PostsApi;

  constructor(api: PostsApi) {
    this.api = api;
  }

  list(query: PostsListQuery): Promise<Result<PostPage>> {
    return safeRepositoryCall(async () => this.api.list(query));
  }

  create(input: PostInput): Promise<Result<Post>> {
    return safeRepositoryCall(async () => this.api.create(input));
  }

  replace(id: string, input: PostInput): Promise<Result<Post>> {
    return safeRepositoryCall(async () => this.api.replace(id, input));
  }

  remove(id: string): Promise<Result<void>> {
    return safeRepositoryCall(async () => {
      await this.api.remove(id);
    });
  }
}
