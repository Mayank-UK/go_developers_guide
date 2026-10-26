import { describe, expect, it } from 'vitest';
import { sqlSafetyDtoFromJson } from './sql-safety-dto';

describe('sqlSafetyDtoFromJson', () => {
  const payload = {
    attacks: [
      {
        input: "x' OR '1'='1",
        intent: 'match every row',
        parameterizedSql: 'SELECT count(*) FROM posts WHERE title = $1',
        rowsMatched: 0,
        postsAfter: 6,
        assembledSql: '…OR…',
        assembledEffect: 'leak',
        verdict: 'inert — bound as data, compared as data',
      },
    ],
    summary: 'separate messages',
  };

  it('builds the DTO from a well-formed payload', () => {
    const dto = sqlSafetyDtoFromJson(payload);
    expect(dto.attacks).toHaveLength(1);
    expect(dto.attacks[0]?.rowsMatched).toBe(0);
    expect(dto.summary).toBe('separate messages');
  });

  it('refuses non-object payloads and missing fields', () => {
    expect(() => sqlSafetyDtoFromJson('nope')).toThrow();
    expect(() => sqlSafetyDtoFromJson({ attacks: [] })).toThrow();
  });
});
