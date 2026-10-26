import type { Result } from '../../core/result';
import type { PostsRepository } from './post-repository';
import type { Post, PostInput } from './post';

/**
 * CreatePostUseCase — create a post from a full draft body. The server
 * validates and normalizes (a broken draft returns as a Failed Result
 * carrying the 422 envelope's code and message); the stored post — id and
 * timestamps included — is the success value.
 */
export class CreatePostUseCase {
  private readonly repository: PostsRepository;

  constructor(repository: PostsRepository) {
    this.repository = repository;
  }

  execute(input: PostInput): Promise<Result<Post>> {
    return this.repository.create(input);
  }
}
