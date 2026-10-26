import { describe, expect, it } from 'vitest';
import { searchDtoFromJson } from './db-search-dto';

describe('searchDtoFromJson', () => {
  const payload = {
    query: 'logging or errors',
    tsQuery: "'log' | 'error'",
    sql: 'SELECT …',
    matches: [{ title: 'Errors are values', rank: '0.0414' }],
    plan: ['Seq Scan on posts  (cost=0.00..1.06 rows=6)'],
    indexDefinition: 'CREATE INDEX posts_search_idx … USING gin …',
    teach: 'stem both sides',
  };

  it('builds the DTO from a well-formed payload', () => {
    const dto = searchDtoFromJson(payload);
    expect(dto.matches[0]?.title).toBe('Errors are values');
    expect(dto.plan).toHaveLength(1);
  });

  it('refuses malformed payloads', () => {
    expect(() => searchDtoFromJson(3)).toThrow();
    expect(() => searchDtoFromJson({ ...payload, plan: [42] })).toThrow();
  });
});
