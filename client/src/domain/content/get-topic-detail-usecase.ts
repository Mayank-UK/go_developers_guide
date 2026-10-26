import type { Result } from '../../core/result';
import type { TopicDetail } from './content';
import type { ContentRepository } from './content-repository';

/**
 * GetTopicDetailUseCase — loads one topic's full detail (sections → blocks)
 * by id for the generic topic screen. Thin orchestration seam over the
 * repository port (react-guide use-case style).
 */
export class GetTopicDetailUseCase {
  private readonly repository: ContentRepository;

  constructor(repository: ContentRepository) {
    this.repository = repository;
  }

  execute(topicId: string): Promise<Result<TopicDetail>> {
    return this.repository.byId(topicId);
  }
}
