import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { ServerFailure } from '../../core/error-handling/failures';
import type { TopicDetail } from './content';
import type { ContentRepository } from './content-repository';
import { GetTopicDetailUseCase } from './get-topic-detail-usecase';

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(byId: Map<string, Result<TopicDetail>>): ContentRepository {
  return {
    list: () => {
      throw new Error('not expected in this use case');
    },
    byId: (topicId: string) => {
      const result = byId.get(topicId);
      if (result === undefined) {
        return Promise.resolve(
          failed(
            new ServerFailure(`Topic '${topicId}' not found`, {
              code: 'not_found',
              status: 404,
            }),
          ),
        );
      }
      return Promise.resolve(result);
    },
  };
}

const topic: TopicDetail = {
  version: 1,
  id: 'routing',
  title: 'Routing',
  group: 'framework',
  order: 7,
  sections: [{ id: 's1', title: 'The section', blocks: [] }],
};

describe('GetTopicDetailUseCase', () => {
  it('returns the topic detail for a known id', async () => {
    const useCase = new GetTopicDetailUseCase(fakeRepository(new Map([[topic.id, success(topic)]])));

    const result = await useCase.execute(topic.id);

    expect(result).toEqual(success(topic));
  });

  it('returns a Failed Result for an unknown id (never throws)', async () => {
    const useCase = new GetTopicDetailUseCase(fakeRepository(new Map()));

    const result = await useCase.execute('missing');

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('not_found');
      expect(result.failure.status).toBe(404);
    }
  });
});
