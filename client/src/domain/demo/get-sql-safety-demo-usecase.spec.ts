import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import { GetSQLSafetyDemoUseCase } from './get-sql-safety-demo-usecase';
import type { SQLSafetyRepository } from './sql-safety-repository';
import type { SQLSafetyDemoData } from './sql-safety';

const fixture: SQLSafetyDemoData = {
  attacks: [
    {
      input: "x' OR '1'='1",
      intent: 'escape the string literal',
      parameterizedSql: 'SELECT count(*) FROM posts WHERE title = $1',
      rowsMatched: 0,
      postsAfter: 6,
      assembledSql: "SELECT count(*) FROM posts WHERE title = 'x' OR '1'='1'",
      assembledEffect: 'every post leaks',
      verdict: 'inert — bound as data, compared as data',
    },
  ],
  summary: 'Separate messages for SQL and values.',
};

function repoWith(result: Promise<Result<SQLSafetyDemoData>>): SQLSafetyRepository {
  return { get: () => result } as SQLSafetyRepository;
}

describe('GetSQLSafetyDemoUseCase', () => {
  it('returns the repository payload on success', async () => {
    const useCase = new GetSQLSafetyDemoUseCase(repoWith(Promise.resolve(success(fixture))));
    const result = await useCase.execute();
    expect(result.ok).toBe(true);
    if (result.ok) expect(result.value.attacks).toHaveLength(1);
  });

  it('passes failures through untouched', async () => {
    const useCase = new GetSQLSafetyDemoUseCase(
      repoWith(Promise.resolve(failed(new NetworkFailure('down')))),
    );
    const result = await useCase.execute();
    expect(result.ok).toBe(false);
  });
});
