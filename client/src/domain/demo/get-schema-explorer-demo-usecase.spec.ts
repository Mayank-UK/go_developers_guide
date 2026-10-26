import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { SchemaExplorerDemoData } from './schema-explorer';
import type { SchemaExplorerRepository } from './schema-explorer-repository';
import { GetSchemaExplorerDemoUseCase } from './get-schema-explorer-demo-usecase';

/**
 * GetSchemaExplorerDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

function fakeRepository(get: Result<SchemaExplorerDemoData>): SchemaExplorerRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: SchemaExplorerDemoData = {
  tables: [
    {
      name: 'posts',
      purpose: "the guide's posts resource",
      columns: [
        { name: 'id', type: 'uuid', nullable: false, default: 'gen_random_uuid()' },
        { name: 'title', type: 'text', nullable: false, default: '' },
      ],
    },
  ],
  constraints: [
    { table: 'post_tags', name: 'post_tags_pkey', kind: 'PRIMARY KEY', definition: 'PRIMARY KEY (post_id, tag_name)' },
  ],
  indexes: [
    {
      table: 'posts',
      name: 'posts_created_at_idx',
      kind: 'btree',
      columns: ['created_at'],
      unique: false,
      primary: false,
      definition: 'CREATE INDEX posts_created_at_idx ON public.posts USING btree (created_at DESC)',
    },
  ],
  history: [
    { version: 0, applied: true, appliedAt: '2026-10-26 21:20:00' },
    { version: 1, applied: true, appliedAt: '2026-10-26 21:20:00' },
  ],
  notes: [{ subject: 'posts_created_at_idx', note: 'explicit btree, DESC: newest-first listing' }],
};

describe('GetSchemaExplorerDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetSchemaExplorerDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetSchemaExplorerDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
