import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { SchemaExplorerApi } from './schema-explorer-api';
import { SchemaExplorerRepositoryImpl } from './schema-explorer-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * SchemaExplorerRepositoryImpl spec — fixture-driven (happy + failure):
 * the happy payload round-trips into the domain shape; network / envelope
 * (the server's honest 503 for an unreachable PostgreSQL) / malformed-
 * payload failures all resolve as Failed Results, never throws.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
  tables: [
    {
      name: 'posts',
      purpose: "the guide's posts resource — topic 17 swaps the in-memory repository for this table",
      columns: [
        { name: 'id', type: 'uuid', nullable: false, default: 'gen_random_uuid()' },
        { name: 'title', type: 'text', nullable: false, default: '' },
      ],
    },
  ],
  constraints: [
    {
      table: 'audit_log',
      name: 'audit_log_post_id_fkey',
      kind: 'FOREIGN KEY',
      definition: 'FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE SET NULL',
    },
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
    { version: 2, applied: true, appliedAt: '2026-10-26 21:20:00' },
  ],
  notes: [
    { subject: 'audit_log_post_id_fkey', note: 'ON DELETE SET NULL: the audit trail outlives its subject' },
  ],
};

function makeRepository(): SchemaExplorerRepositoryImpl {
  return new SchemaExplorerRepositoryImpl(new SchemaExplorerApi(createAppConfig()));
}

describe('SchemaExplorerRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/db/schema-explorer', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.tables[0]?.name).toBe('posts');
      expect(result.value.tables[0]?.columns[0]?.default).toBe('gen_random_uuid()');
      expect(result.value.constraints[0]?.definition).toContain('ON DELETE SET NULL');
      expect(result.value.indexes[0]?.kind).toBe('btree');
      expect(result.value.history).toHaveLength(3);
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });

  it('guards the server\u2019s honest 503 into a Failed Result with its diagnostics', async () => {
    stubApiFetch([
      {
        url: '/api/demo/db/schema-explorer',
        status: 503,
        body: {
          error: {
            code: 'service_unavailable',
            message: 'The schema explorer reads the guide\u2019s live PostgreSQL, which is not reachable right now.',
            requestId: 'req-9',
          },
        },
      },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure(
          'The schema explorer reads the guide\u2019s live PostgreSQL, which is not reachable right now.',
          {
            code: 'service_unavailable',
            status: 503,
            requestId: 'req-9',
          },
        ),
      ),
    );
  });

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // Missing the constraints array — the DTO validator must reject it.
      { url: '/api/demo/db/schema-explorer', body: { tables: [] } },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('The server response did not match the expected shape.', {
          code: 'invalid_response',
          status: 200,
        }),
      ),
    );
  });

  it('rejects an unknown constraint kind instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/db/schema-explorer',
        body: {
          ...demoPayload,
          constraints: [{ table: 't', name: 'n', kind: 'mystery', definition: 'd' }],
        },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });
});
