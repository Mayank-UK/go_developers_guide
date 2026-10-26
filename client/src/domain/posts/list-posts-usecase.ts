import type { Result } from '../../core/result';
import type { PostsRepository } from './post-repository';
import type { PostPage, PostsListQuery } from './post';

/**
 * ListPostsUseCase — one honest page of the posts collection for the
 * workbench's current controls (page, limit, tag filter, sort). The page
 * metadata (totals, neighbor flags) is part of the value: an empty page
 * past the end is a success, not an error.
 */
export class ListPostsUseCase {
  private readonly repository: PostsRepository;

  constructor(repository: PostsRepository) {
    this.repository = repository;
  }

  execute(query: PostsListQuery): Promise<Result<PostPage>> {
    return this.repository.list(query);
  }
}
