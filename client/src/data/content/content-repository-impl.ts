import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ContentIndex, TopicDetail } from '../../domain/content/content';
import type { ContentRepository } from '../../domain/content/content-repository';
import { toContentIndex, toTopicDetail } from './content-dto';
import type { ContentApi } from './content-api';

/**
 * ContentRepositoryImpl — fetches content DTOs from the API and maps them to
 * domain models. Every expected failure (network down, error envelope,
 * malformed payload) arrives as a typed AppException from the API layer and is
 * mapped to a Failure wrapped in a `Result` — the method never throws, and no
 * rejection goes unhandled.
 */
export class ContentRepositoryImpl implements ContentRepository {
  private readonly api: ContentApi;

  constructor(api: ContentApi) {
    this.api = api;
  }

  list(): Promise<Result<ContentIndex>> {
    return safeRepositoryCall(async () => toContentIndex(await this.api.list()));
  }

  byId(topicId: string): Promise<Result<TopicDetail>> {
    return safeRepositoryCall(async () => toTopicDetail(await this.api.byId(topicId)));
  }
}
