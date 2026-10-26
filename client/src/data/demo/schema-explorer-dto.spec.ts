import { describe, expect, it } from 'vitest';
import { schemaExplorerDtoFromJson, toSchemaExplorerDemo } from './schema-explorer-dto';
import type { SchemaExplorerDemoData } from '../../domain/demo/schema-explorer';

/**
 * SchemaExplorerDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). `purpose` and
 * column `default` may be EMPTY STRINGS (data, not absence); `appliedAt`
 * varies per environment by design — asserted by shape, never by literal.
 */

const wellFormed: SchemaExplorerDemoData = {
  tables: [
    {
      name: 'posts',
      purpose: "the guide's posts resource — topic 17 swaps the in-memory repository for this table",
      columns: [
        { name: 'id', type: 'uuid', nullable: false, default: 'gen_random_uuid()' },
        { name: 'title', type: 'text', nullable: false, default: '' },
        { name: 'post_id_ref', type: 'uuid', nullable: true, default: '' },
      ],
    },
    { name: 'mystery', purpose: '', columns: [] },
  ],
  constraints: [
    { table: 'post_tags', name: 'post_tags_pkey', kind: 'PRIMARY KEY', definition: 'PRIMARY KEY (post_id, tag_name)' },
    { table: 'audit_log', name: 'audit_log_post_id_fkey', kind: 'FOREIGN KEY', definition: 'FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE SET NULL' },
    { table: 'posts', name: 'posts_title_check', kind: 'CHECK', definition: 'CHECK (length(title) BETWEEN 1 AND 200)' },
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
    { table: 'posts', name: 'posts_pkey', kind: 'btree', columns: ['id'], unique: true, primary: true, definition: 'CREATE UNIQUE INDEX posts_pkey ON public.posts USING btree (id)' },
  ],
  history: [
    { version: 0, applied: true, appliedAt: '2026-10-26 21:20:00' },
    { version: 1, applied: true, appliedAt: '2026-10-26 21:20:00.894163' },
    { version: 2, applied: true, appliedAt: '2026-10-26 21:20:00.950326' },
  ],
  notes: [
    { subject: 'posts_created_at_idx', note: 'explicit btree, DESC: the API\u2019s default listing is newest-first' },
  ],
};

describe('schemaExplorerDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(schemaExplorerDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps empty purpose and empty default as data (present-but-empty passes)', () => {
    const parsed = schemaExplorerDtoFromJson(wellFormed);

    expect(parsed.tables[1]?.purpose).toBe('');
    expect(parsed.tables[0]?.columns[1]?.default).toBe('');
    expect(parsed.tables[0]?.columns[2]?.nullable).toBe(true);
  });

  it('accepts appliedAt stamps of any textual shape (per-environment data)', () => {
    const parsed = schemaExplorerDtoFromJson(wellFormed);

    expect(parsed.history.map((row) => typeof row.appliedAt === 'string')).toEqual([true, true, true]);
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing tables', { ...wellFormed, tables: undefined }],
    ['tables not an array', { ...wellFormed, tables: 'nope' }],
    ['a non-object table', { ...wellFormed, tables: ['nope'] }],
    ['missing table name', { ...wellFormed, tables: [{ purpose: 'p', columns: [] }] }],
    ['missing purpose (omission is not empty)', { ...wellFormed, tables: [{ name: 'posts', columns: [] }] }],
    ['null purpose (null is not omission)', { ...wellFormed, tables: [{ name: 'posts', purpose: null, columns: [] }] }],
    ['missing columns', { ...wellFormed, tables: [{ name: 'posts', purpose: 'p' }] }],
    ['a non-object column', { ...wellFormed, tables: [{ name: 'posts', purpose: 'p', columns: ['x'] }] }],
    ['missing nullable', { ...wellFormed, tables: [{ name: 'posts', purpose: 'p', columns: [{ name: 'id', type: 'uuid', default: '' }] }] }],
    ['missing default', { ...wellFormed, tables: [{ name: 'posts', purpose: 'p', columns: [{ name: 'id', type: 'uuid', nullable: false }] }] }],
    ['missing constraints', { ...wellFormed, constraints: undefined }],
    ['unknown constraint kind', { ...wellFormed, constraints: [{ table: 't', name: 'n', kind: 'WIZARDRY', definition: 'd' }] }],
    ['missing constraint definition', { ...wellFormed, constraints: [{ table: 't', name: 'n', kind: 'CHECK' }] }],
    ['missing indexes', { ...wellFormed, indexes: undefined }],
    ['a non-string index column', { ...wellFormed, indexes: [{ ...wellFormed.indexes[0]!, columns: ['created_at', 42] }] }],
    ['a non-boolean index primary flag', { ...wellFormed, indexes: [{ ...wellFormed.indexes[0]!, primary: 'yes' }] }],
    ['missing history', { ...wellFormed, history: undefined }],
    ['a non-number history version', { ...wellFormed, history: [{ version: 'one', applied: true, appliedAt: 'x' }] }],
    ['missing notes', { ...wellFormed, notes: undefined }],
    ['an empty note subject', { ...wellFormed, notes: [{ subject: '', note: 'n' }] }],
  ])('throws for %s', (_label, body) => {
    expect(() => schemaExplorerDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = schemaExplorerDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      tables: wellFormed.tables.map((table) => ({ ...table, sneaky: true })),
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toSchemaExplorerDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toSchemaExplorerDemo(wellFormed)).toBe(wellFormed);
  });
});
