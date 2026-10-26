import type { Result } from '../../core/result';
import type { PostsRepository } from './post-repository';

/**
 * DeletePostUseCase — remove a post under the server's strict stance: a
 * miss (already gone, mistyped id) is a Failed Result carrying the 404
 * envelope, never a silent success — the workbench shows that miss
 * honestly.
 */
export class DeletePostUseCase {
  private readonly repository: PostsRepository;

  constructor(repository: PostsRepository) {
    this.repository = repository;
  }

  execute(id: string): Promise<Result<void>> {
    return this.repository.remove(id);
  }
}
