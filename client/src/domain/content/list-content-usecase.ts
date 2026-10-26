import type { Result } from '../../core/result';
import type { ContentIndex } from './content';
import type { ContentRepository } from './content-repository';

/**
 * ListContentUseCase — loads the guide's curriculum index (groups + topic
 * summaries) for the home screen's topic map. A thin orchestration seam over
 * the repository port (react-guide use-case style): the boundary is where
 * cross-cutting policy (caching, combining sources, business rules) would
 * live, keeping the repository focused on a single source.
 */
export class ListContentUseCase {
  private readonly repository: ContentRepository;

  constructor(repository: ContentRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ContentIndex>> {
    return this.repository.list();
  }
}
