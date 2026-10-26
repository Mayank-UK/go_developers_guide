import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { PostsWorkbenchRepository } from '../../domain/demo/posts-workbench-repository';
import type { PostsWorkbenchDemoData } from '../../domain/demo/posts-workbench';
import type { PostsWorkbenchApi } from './posts-workbench-api';

/**
 * PostsWorkbenchRepositoryImpl — adapts the contract-card API to the
 * domain port. Every thrown transport error (network, non-2xx envelope,
 * malformed payload) is guarded by safeRepositoryCall into a Failed
 * Result, so the demo component never needs try/catch.
 */
export class PostsWorkbenchRepositoryImpl implements PostsWorkbenchRepository {
  private readonly api: PostsWorkbenchApi;

  constructor(api: PostsWorkbenchApi) {
    this.api = api;
  }

  async get(): Promise<Result<PostsWorkbenchDemoData>> {
    return safeRepositoryCall(async () => this.api.get());
  }
}
