import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { RuntimeTypesApi } from './runtime-types-api';
import { RuntimeTypesRepositoryImpl } from './runtime-types-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * RuntimeTypesRepositoryImpl spec — fixture-driven (happy + failure): the
 * happy payload round-trips into the domain shape; network / envelope /
 * malformed-payload failures all resolve as Failed Results, never throws.
 */

/** A minimal but structurally complete wire payload. */
const demoPayload = {
  inspections: [
    {
      expression: 'content.Topic{}',
      type: 'guide/content.Topic',
      kind: 'struct',
      pkgPath: 'guide/content',
      sizeBytes: 96,
    },
    {
      expression: 'content.NewTextBlock',
      type: 'func(string, string, string) (content.TextBlock, error)',
      kind: 'func',
      pkgPath: 'guide/content',
      sizeBytes: 8,
    },
    { expression: '43', type: 'int', kind: 'int', pkgPath: '', sizeBytes: 8 },
  ],
  fields: [
    { name: 'ID', type: 'string', jsonTag: 'id' },
    { name: 'Title', type: 'string', jsonTag: 'title' },
    { name: 'Lead', type: 'string', jsonTag: 'lead,omitempty' },
    { name: 'Blocks', type: '[]content.Block', jsonTag: 'blocks' },
  ],
  assertions: [
    { expression: 'blocks[0].(type)', dynamicType: 'content.TextBlock', ok: true, blockID: 'demo:text' },
    { expression: 'blocks[0].(content.CodeBlock)', dynamicType: 'content.CodeBlock', ok: false, blockID: '' },
  ],
  zeroValues: [
    { declaration: 'var id string', type: 'string', display: '""', isZero: true },
    { declaration: 'var sections []content.Section', type: '[]content.Section', display: 'nil or 0', isZero: true },
  ],
  enforcement: {
    function: 'repeat',
    signature: 'func repeat(text string, times int) string',
    call: 'repeat(3.0, 2)',
    compilerVerdict:
      'cannot use 3.0 (untyped float constant 3) as string value in argument to repeat',
    runtimeOutcome:
      'none — the program containing the call never builds; the typed call repeat("go", 3) returns "gogogo"',
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

function makeRepository(): RuntimeTypesRepositoryImpl {
  return new RuntimeTypesRepositoryImpl(new RuntimeTypesApi(createAppConfig()));
}

describe('RuntimeTypesRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/go/runtime-types', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.enforcement.compilerVerdict).toContain('cannot use 3.0');
      expect(result.value.inspections[2]?.pkgPath).toBe('');
      expect(result.value.assertions[1]?.ok).toBe(false);
      expect(result.value.fields[2]?.jsonTag).toBe('lead,omitempty');
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

  it('guards an error envelope into a Failed Result with its diagnostics', async () => {
    stubApiFetch([
      {
        url: '/api/demo/go/runtime-types',
        status: 503,
        body: { error: { code: 'service_unavailable', message: 'calm down', requestId: 'req-9' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('calm down', {
          code: 'service_unavailable',
          status: 503,
          requestId: 'req-9',
        }),
      ),
    );
  });

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // Missing every section — the DTO validator must reject it.
      { url: '/api/demo/go/runtime-types', body: { inspections: 'nope' } },
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

  it('rejects a non-boolean ok flag instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/go/runtime-types',
        body: { ...demoPayload, assertions: [{ expression: 'x', dynamicType: 'y', ok: 'true', blockID: 'z' }] },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });
});
