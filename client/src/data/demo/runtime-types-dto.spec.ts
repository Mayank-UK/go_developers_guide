import { describe, expect, it } from 'vitest';
import { runtimeTypesDtoFromJson, toRuntimeTypesDemo } from './runtime-types-dto';
import type { RuntimeTypesDemoData } from '../../domain/demo/runtime-types';

/**
 * RuntimeTypesDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). Empty-string
 * fields that ARE data — a predeclared type's pkgPath, a missed
 * assertion's blockID — must survive validation.
 */

const wellFormed: RuntimeTypesDemoData = {
  inspections: [
    {
      expression: 'content.Topic{}',
      type: 'guide/content.Topic',
      kind: 'struct',
      pkgPath: 'guide/content',
      sizeBytes: 96,
    },
    { expression: `"go-language"`, type: 'string', kind: 'string', pkgPath: '', sizeBytes: 16 },
  ],
  fields: [
    { name: 'ID', type: 'string', jsonTag: 'id' },
    { name: 'Lead', type: 'string', jsonTag: 'lead,omitempty' },
  ],
  assertions: [
    { expression: 'blocks[0].(type)', dynamicType: 'content.TextBlock', ok: true, blockID: 'demo:text' },
    { expression: 'blocks[0].(content.CodeBlock)', dynamicType: 'content.CodeBlock', ok: false, blockID: '' },
  ],
  zeroValues: [
    { declaration: 'var present bool', type: 'bool', display: 'false', isZero: true },
  ],
  enforcement: {
    function: 'repeat',
    signature: 'func repeat(text string, times int) string',
    call: 'repeat(3.0, 2)',
    compilerVerdict:
      'cannot use 3.0 (untyped float constant 3) as string value in argument to repeat',
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

describe('runtimeTypesDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(runtimeTypesDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps empty strings that are data: predeclared pkgPath, missed blockID', () => {
    const parsed = runtimeTypesDtoFromJson(wellFormed);

    expect(parsed.inspections[1]?.pkgPath).toBe('');
    expect(parsed.assertions[1]?.blockID).toBe('');
    expect(parsed.assertions[1]?.ok).toBe(false);
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing inspections', { ...wellFormed, inspections: undefined }],
    ['missing fields', { ...wellFormed, fields: undefined }],
    ['missing assertions', { ...wellFormed, assertions: undefined }],
    ['missing zeroValues', { ...wellFormed, zeroValues: undefined }],
    ['missing enforcement', { ...wellFormed, enforcement: undefined }],
    ['missing boundary', { ...wellFormed, boundary: undefined }],
    ['missing fieldsNote', { ...wellFormed, fieldsNote: undefined }],
    ['missing assertionsNote', { ...wellFormed, assertionsNote: undefined }],
    ['inspections not an array', { ...wellFormed, inspections: 'nope' }],
    ['a non-object inspection row', { ...wellFormed, inspections: ['nope'] }],
    ['missing sizeBytes', { ...wellFormed, inspections: [{ ...wellFormed.inspections[0]!, sizeBytes: undefined }] }],
    ['NaN sizeBytes', { ...wellFormed, inspections: [{ ...wellFormed.inspections[0]!, sizeBytes: Number.NaN }] }],
    ['missing jsonTag', { ...wellFormed, fields: [{ name: 'ID', type: 'string' }] }],
    ['missing ok flag', { ...wellFormed, assertions: [{ ...wellFormed.assertions[0]!, ok: undefined }] }],
    ['non-boolean isZero', { ...wellFormed, zeroValues: [{ ...wellFormed.zeroValues[0]!, isZero: 'yes' }] }],
    ['missing blockID (absent is not empty)', { ...wellFormed, assertions: [{ ...wellFormed.assertions[1]!, blockID: undefined }] }],
    ['missing compilerVerdict', { ...wellFormed, enforcement: { ...wellFormed.enforcement, compilerVerdict: undefined } }],
    ['non-object boundary', { ...wellFormed, boundary: 'rejected' }],
    ['empty boundary message', { ...wellFormed, boundary: { ...wellFormed.boundary, message: '' } }],
  ])('throws for %s', (_label, body) => {
    expect(() => runtimeTypesDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = runtimeTypesDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      enforcement: { ...wellFormed.enforcement, sneaky: true },
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toRuntimeTypesDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toRuntimeTypesDemo(wellFormed)).toBe(wellFormed);
  });
});
