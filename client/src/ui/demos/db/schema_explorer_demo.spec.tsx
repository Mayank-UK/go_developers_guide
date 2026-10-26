import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SchemaExplorerDemo } from './schema_explorer_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetSchemaExplorerDemoUseCase,
  SCHEMA_EXPLORER_DEMO_USE_CASE,
} from '../../../domain/demo/get-schema-explorer-demo-usecase';
import type { SchemaExplorerDemoData } from '../../../domain/demo/schema-explorer';

/**
 * SchemaExplorerDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the migration ledger renders, tables with their columns and real
 * types render, constraint definitions (ON DELETE included) render,
 * indexes with kinds render, rationale notes render, and failures —
 * including the server's honest 503 for an unreachable PostgreSQL —
 * degrade to the shared error panel with recovery.
 */

const fixture: SchemaExplorerDemoData = {
  tables: [
    {
      name: 'audit_log',
      purpose: 'append-only history of post changes, written inside the changing transaction',
      columns: [
        { name: 'id', type: 'uuid', nullable: false, default: 'gen_random_uuid()' },
        { name: 'post_id', type: 'uuid', nullable: true, default: '' },
        { name: 'action', type: 'text', nullable: false, default: '' },
      ],
    },
    {
      name: 'posts',
      purpose: "the guide's posts resource — topic 17 swaps the in-memory repository for this table",
      columns: [
        { name: 'id', type: 'uuid', nullable: false, default: 'gen_random_uuid()' },
        { name: 'created_at', type: 'timestamp with time zone', nullable: false, default: 'now()' },
      ],
    },
  ],
  constraints: [
    { table: 'audit_log', name: 'audit_log_post_id_fkey', kind: 'FOREIGN KEY', definition: 'FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE SET NULL' },
    { table: 'post_tags', name: 'post_tags_pkey', kind: 'PRIMARY KEY', definition: 'PRIMARY KEY (post_id, tag_name)' },
    { table: 'posts', name: 'posts_title_check', kind: 'CHECK', definition: 'CHECK (length(title) BETWEEN 1 AND 200)' },
  ],
  indexes: [
    { table: 'posts', name: 'posts_created_at_idx', kind: 'btree', columns: ['created_at'], unique: false, primary: false, definition: 'CREATE INDEX posts_created_at_idx ON public.posts USING btree (created_at DESC)' },
    { table: 'posts', name: 'posts_pkey', kind: 'btree', columns: ['id'], unique: true, primary: true, definition: 'CREATE UNIQUE INDEX posts_pkey ON public.posts USING btree (id)' },
  ],
  history: [
    { version: 0, applied: true, appliedAt: '2026-10-26 21:20:00' },
    { version: 1, applied: true, appliedAt: '2026-10-26 21:20:00' },
    { version: 2, applied: true, appliedAt: '2026-10-26 21:20:00' },
  ],
  notes: [
    { subject: 'posts_created_at_idx', note: 'explicit btree, DESC: the API\u2019s default listing is newest-first' },
    { subject: 'audit_log_post_id_fkey', note: 'ON DELETE SET NULL: the audit trail outlives its subject' },
  ],
};

function stubUseCase(execute: () => Promise<Result<SchemaExplorerDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    SCHEMA_EXPLORER_DEMO_USE_CASE,
    { execute } as unknown as GetSchemaExplorerDemoUseCase,
  );
}

describe('SchemaExplorerDemo', () => {
  it('renders loading first, then the live schema', async () => {
    stubUseCase(async () => success(fixture));

    render(<SchemaExplorerDemo />);

    expect(screen.getByTestId('schema-explorer-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('schema-explorer-ready')).toBeInTheDocument());

    // The migration ledger, baseline row included.
    const ledger = screen.getByTestId('se-migrations');
    expect(ledger).toHaveTextContent('v0');
    expect(ledger).toHaveTextContent('v2');

    // Tables with columns, real types, nullability, defaults.
    const tables = screen.getByTestId('se-tables');
    expect(tables).toHaveTextContent('posts');
    expect(tables).toHaveTextContent('audit_log');
    expect(tables).toHaveTextContent('timestamp with time zone');
    expect(tables).toHaveTextContent('gen_random_uuid()');
    expect(tables).toHaveTextContent('NOT NULL');

    // Constraint definitions verbatim from pg_get_constraintdef.
    const constraints = screen.getByTestId('se-constraints');
    expect(constraints).toHaveTextContent('ON DELETE SET NULL');
    expect(constraints).toHaveTextContent('PRIMARY KEY (post_id, tag_name)');
    expect(constraints).toHaveTextContent('CHECK (length(title) BETWEEN 1 AND 200)');

    // Indexes with kinds and columns.
    const indexes = screen.getByTestId('se-indexes');
    expect(indexes).toHaveTextContent('btree');
    expect(indexes).toHaveTextContent('posts_created_at_idx');
    expect(indexes).toHaveTextContent('(created_at)');

    // Curated rationale notes.
    expect(screen.getByTestId('se-notes')).toHaveTextContent('audit trail outlives its subject');
  });

  it('renders nullable columns distinctly from NOT NULL ones', async () => {
    stubUseCase(async () => success(fixture));

    render(<SchemaExplorerDemo />);

    await waitFor(() => expect(screen.getByTestId('se-tables')).toBeInTheDocument());
    const tables = screen.getByTestId('se-tables');
    expect(tables).toHaveTextContent('NULL');
  });

  it('degrades to the error panel on the server\u2019s honest 503, with recovery', async () => {
    const failure = new ServerFailure(
      'The schema explorer reads the guide\u2019s live PostgreSQL, which is not reachable right now.',
      { code: 'service_unavailable', status: 503, requestId: 'req-503' },
    );
    stubUseCase(async () => failed(failure));

    render(<SchemaExplorerDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');
    expect(panel).toHaveTextContent('not reachable right now');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('schema-explorer-ready')).toBeInTheDocument());
  });
});
