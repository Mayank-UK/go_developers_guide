import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { RuntimeTypesDemoData } from './runtime-types';
import type { RuntimeTypesRepository } from './runtime-types-repository';
import { GetRuntimeTypesDemoUseCase } from './get-runtime-types-demo-usecase';

/**
 * GetRuntimeTypesDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<RuntimeTypesDemoData>): RuntimeTypesRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: RuntimeTypesDemoData = {
  inspections: [
    { expression: `content.Topic{}`, type: 'guide/content.Topic', kind: 'struct', pkgPath: 'guide/content', sizeBytes: 96 },
  ],
  fields: [{ name: 'ID', type: 'string', jsonTag: 'id' }],
  assertions: [
    { expression: 'blocks[0].(type)', dynamicType: 'content.TextBlock', ok: true, blockID: 'demo:text' },
  ],
  zeroValues: [{ declaration: 'var id string', type: 'string', display: '""', isZero: true }],
  enforcement: {
    function: 'repeat',
    signature: 'func repeat(text string, times int) string',
    call: 'repeat(3.0, 2)',
    compilerVerdict: 'cannot use 3.0 (untyped float constant 3) as string value in argument to repeat',
    runtimeOutcome: 'none — the program containing the call never builds',
  },
  boundary: {
    decodeTarget: 'content.TextBlock',
    badInput: '{"kind":"text","id":42,"text":"Contraband"}',
    outcome: 'rejected',
    errorType: '*json.UnmarshalTypeError',
    message: 'json: cannot unmarshal number into Go struct field TextBlock.id of type string',
  },
  fieldsNote: 'The tag column is read at runtime by reflect.',
  assertionsNote: 'The comma-ok row reports false instead of panicking.',
};

describe('GetRuntimeTypesDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetRuntimeTypesDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetRuntimeTypesDemoUseCase(
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
