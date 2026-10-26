import type { Result } from '../../core/result';
import type { ContentIndex, TopicDetail } from './content';

/**
 * ContentRepository — the domain-facing port for reading the guide's teaching
 * content. Use cases and views depend on this interface only; the adapter that
 * talks HTTP lives in data/content and is wired at the composition root
 * (core/container.ts).
 */
export interface ContentRepository {
  /** The guide's curriculum index: groups + topic summaries. */
  list(): Promise<Result<ContentIndex>>;

  /** One topic's full detail (sections → blocks) by its id. */
  byId(topicId: string): Promise<Result<TopicDetail>>;
}
