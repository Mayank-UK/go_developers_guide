import { describe, expect, it } from 'vitest';
import { sqlLabDtoFromJson } from './sql-lab-dto';

describe('sqlLabDtoFromJson', () => {
  const payload = {
    queries: [
      {
        id: 'tag-totals',
        title: 'A CTE chain',
        sql: 'WITH …',
        teach: 'name the intermediate',
        columns: ['tag', 'posts'],
        rows: [['api', 2], ['go', 4]],
      },
    ],
  };

  it('builds the DTO from a well-formed payload', () => {
    const dto = sqlLabDtoFromJson(payload);
    expect(dto.queries[0]?.rows[1]).toEqual(['go', 4]);
  });

  it('refuses malformed rows and payloads', () => {
    expect(() => sqlLabDtoFromJson(null)).toThrow();
    expect(() =>
      sqlLabDtoFromJson({ queries: [{ ...payload.queries[0], rows: [[true]] }] }),
    ).toThrow();
  });
});
