import { describe, expect, it } from 'vitest';
import { ORMProbeRepositoryImpl } from './orm-probe-repository-impl';
import type { ORMProbeApi } from './orm-probe-api';
import type { ORMProbeDemoData } from '../../domain/demo/orm-probe';

const fixture = {
  dataset: { posts: 12, tags: 5, edges: 24 },
  trials: [],
  resultsAgree: true,
  unitOfWork: {
    repositories: ['GormPostsRepository', 'GormTagsRepository'],
    steps: [],
    hookApplied: true,
    postsBefore: 12,
    postsAfter: 12,
    rolledBack: true,
  },
} as ORMProbeDemoData;

describe('ORMProbeRepositoryImpl', () => {
  it('wraps the api payload in a success Result', async () => {
    const repo = new ORMProbeRepositoryImpl({ get: async () => fixture } as unknown as ORMProbeApi);
    expect((await repo.get()).ok).toBe(true);
  });

  it('guards api rejections into failed Results', async () => {
    const repo = new ORMProbeRepositoryImpl({
      get: async () => {
        throw new Error('boom');
      },
    } as unknown as ORMProbeApi);
    expect((await repo.get()).ok).toBe(false);
  });
});
