import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { ValidationArenaDemoData } from './validation-arena';
import type { ValidationArenaRepository } from './validation-arena-repository';
import { GetValidationArenaDemoUseCase } from './get-validation-arena-demo-usecase';

/**
 * GetValidationArenaDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<ValidationArenaDemoData>): ValidationArenaRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: ValidationArenaDemoData = {
  subject: {
    name: 'FeedbackForm',
    source: 'server/internal/domain/demos/validation_arena.go',
    fields: [
      { name: 'title', type: 'string', rule: 'required; trimmed non-blank; at most 80 runes', required: true },
      { name: 'body', type: 'string', rule: 'optional; at most 280 runes', required: false },
      { name: 'score', type: 'int', rule: 'required; whole number 1–5', required: true },
      { name: 'email', type: 'string', rule: 'required; one @ with text on both sides', required: true },
    ],
  },
  layers: [
    { id: 'decode', title: 'Layer one', tool: 'encoding/json', summary: 'Shape and types.' },
    { id: 'rules', title: 'Layer two', tool: 'plain Go functions', summary: 'The backbone.' },
    { id: 'library', title: 'Layer three', tool: 'go-playground/validator', summary: 'By comparison.' },
  ],
  candidates: [
    {
      id: 'valid-complete',
      label: 'A well-formed submission',
      expects: 'The baseline.',
      body: '{"title":"Loved the goroutines topic"}',
      decode: { status: 'accepted' },
      validate: { status: 'validated' },
      response: { status: 200, validated: { title: 'Loved the goroutines topic', body: '', score: 5, email: 'reader@example.com' } },
    },
  ],
  comparison: {
    library: 'github.com/go-playground/validator/v10',
    imported: false,
    rows: [{ field: 'score', tag: 'required,gte=1,lte=5', libraryRule: 'a numeric range', handRolled: 'whole number 1–5' }],
    note: 'The tags are inert without a library that reads them.',
  },
  envelope: {
    candidateId: 'collect-every-issue',
    status: 422,
    body: { error: { code: 'validation_failed', message: 'Feedback submission failed validation.' } },
    anatomy: [{ path: 'error.code', value: 'validation_failed', why: 'the taxonomy bucket' }],
  },
  reasoning: 'Twelve candidate bodies, each run through the real two-layer pipeline.',
};

describe('GetValidationArenaDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetValidationArenaDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetValidationArenaDemoUseCase(
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
