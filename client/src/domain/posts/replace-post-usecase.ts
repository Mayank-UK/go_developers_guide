import type { Result } from '../../core/result';
import type { PostsRepository } from './post-repository';
import type { Post, PostInput } from './post';

/**
 * ReplacePostUseCase — full replacement (PUT semantics): the input is the
 * post's whole next state, so omitted fields are cleared, not merged. A
 * missing id returns as a Failed Result carrying the 404 envelope.
 */
export class ReplacePostUseCase {
  private readonly repository: PostsRepository;

  constructor(repository: PostsRepository) {
    this.repository = repository;
  }

  execute(id: string, input: PostInput): Promise<Result<Post>> {
    return this.repository.replace(id, input);
  }
}
