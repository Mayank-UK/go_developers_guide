import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { InterfaceMatrixDemoData } from './interface-matrix';
import type { InterfaceMatrixRepository } from './interface-matrix-repository';
import { GetInterfaceMatrixDemoUseCase } from './get-interface-matrix-demo-usecase';

/**
 * GetInterfaceMatrixDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<InterfaceMatrixDemoData>): InterfaceMatrixRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: InterfaceMatrixDemoData = {
  interfaces: [
    { name: 'io.Reader', method: 'Read([]byte) (int, error)', pkg: 'io', note: 'The canon.' },
  ],
  rows: [
    {
      expression: '*core.Error',
      type: '*core.Error',
      kind: 'ptr',
      methods: ['Error', 'Is', 'Unwrap'],
      cells: [{ iface: 'io.Reader', satisfied: false }],
    },
  ],
  embeddings: [
    {
      expression: 'type taggedReader struct { *strings.Reader; tag string }',
      embeds: '*strings.Reader (concrete type)',
      type: 'demos.taggedReader',
      iface: 'io.Reader',
      satisfied: true,
      note: 'Read arrives by promotion.',
    },
  ],
  typeSwitch: [
    { input: '*bytes.Buffer', matchedCase: 'case io.Reader', result: 'io.Reader case — dynamic type *bytes.Buffer', note: 'first match wins' },
  ],
  compileProofs: [{ line: '_ io.Reader     = (*bytes.Buffer)(nil)', pins: 'io.Reader × *bytes.Buffer' }],
};

describe('GetInterfaceMatrixDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetInterfaceMatrixDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetInterfaceMatrixDemoUseCase(
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
