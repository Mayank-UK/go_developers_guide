import { describe, expect, it } from 'vitest';
import { interfaceMatrixDtoFromJson, toInterfaceMatrixDemo } from './interface-matrix-dto';
import type { InterfaceMatrixDemoData } from '../../domain/demo/interface-matrix';

/**
 * InterfaceMatrixDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). The fixture
 * mirrors the Go builder's real output: verdicts from actual method sets,
 * a sealed-hierarchy row with an EMPTY exported-methods list, and a
 * type-switch row whose optional note is genuinely absent.
 */

const wellFormed: InterfaceMatrixDemoData = {
  interfaces: [
    { name: 'fmt.Stringer', method: 'String() string', pkg: 'fmt', note: "One method — fmt's whole formatting world keys off it." },
    { name: 'error', method: 'Error() string', pkg: 'builtin', note: "The language's own interface." },
    { name: 'io.Reader', method: 'Read([]byte) (int, error)', pkg: 'io', note: 'The canon.' },
    { name: 'http.Handler', method: 'ServeHTTP(http.ResponseWriter, *http.Request)', pkg: 'net/http', note: 'One method, the entire HTTP layer.' },
    { name: 'content.Block', method: 'blockKind() string', pkg: 'guide/internal/content', note: 'Unexported method — a sealed hierarchy.' },
  ],
  rows: [
    {
      expression: '*core.Error',
      type: '*core.Error',
      kind: 'ptr',
      methods: ['Error', 'Is', 'Unwrap'],
      cells: [
        { iface: 'fmt.Stringer', satisfied: false },
        { iface: 'error', satisfied: true },
        { iface: 'io.Reader', satisfied: false },
        { iface: 'http.Handler', satisfied: false },
        { iface: 'content.Block', satisfied: false },
      ],
    },
    {
      expression: 'core.Error',
      type: 'core.Error',
      kind: 'struct',
      methods: [],
      cells: [
        { iface: 'fmt.Stringer', satisfied: false },
        { iface: 'error', satisfied: false },
        { iface: 'io.Reader', satisfied: false },
        { iface: 'http.Handler', satisfied: false },
        { iface: 'content.Block', satisfied: false },
      ],
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
    { input: 'content.Topic{}', matchedCase: 'default', result: 'unhandled content.Topic' },
    {
      input: '*bytes.Buffer',
      matchedCase: 'case io.Reader',
      result: 'io.Reader case — dynamic type *bytes.Buffer',
      note: '*bytes.Buffer satisfies fmt.Stringer too — cases run top to bottom, and the first match wins.',
    },
  ],
  compileProofs: [
    { line: '_ error         = (*core.Error)(nil)', pins: 'error × *core.Error' },
    { line: '_ io.Reader     = taggedReader{}', pins: 'io.Reader × taggedReader (promoted)' },
  ],
};

describe('interfaceMatrixDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(interfaceMatrixDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps the honest empties: empty method list, absent optional note', () => {
    const parsed = interfaceMatrixDtoFromJson(wellFormed);

    // The pointer-receiver gap: the VALUE row's exported method set is [].
    expect(parsed.rows[1]?.methods).toEqual([]);
    // The default-case row omits its optional note entirely (omitempty).
    expect(parsed.typeSwitch[0]?.note).toBeUndefined();
    expect(parsed.typeSwitch[1]?.note).toBe(
      '*bytes.Buffer satisfies fmt.Stringer too — cases run top to bottom, and the first match wins.',
    );
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing interfaces', { ...wellFormed, interfaces: undefined }],
    ['missing rows', { ...wellFormed, rows: undefined }],
    ['missing embeddings', { ...wellFormed, embeddings: undefined }],
    ['missing typeSwitch', { ...wellFormed, typeSwitch: undefined }],
    ['missing compileProofs', { ...wellFormed, compileProofs: undefined }],
    ['rows not an array', { ...wellFormed, rows: 'nope' }],
    ['a non-object row', { ...wellFormed, rows: ['nope'] }],
    ['missing cells', { ...wellFormed, rows: [{ ...wellFormed.rows[0]!, cells: undefined }] }],
    ['missing satisfied flag', { ...wellFormed, rows: [{ ...wellFormed.rows[0]!, cells: [{ iface: 'error' }] }] }],
    ['non-boolean satisfied', { ...wellFormed, rows: [{ ...wellFormed.rows[0]!, cells: [{ iface: 'error', satisfied: 'yes' }] }] }],
    ['empty method string inside the list', { ...wellFormed, rows: [{ ...wellFormed.rows[0]!, methods: ['Error', ''] }] }],
    ['methods not an array', { ...wellFormed, rows: [{ ...wellFormed.rows[0]!, methods: 'Error' }] }],
    ['empty interface name', { ...wellFormed, interfaces: [{ ...wellFormed.interfaces[0]!, name: '' }] }],
    ['missing embeds field', { ...wellFormed, embeddings: [{ expression: 'x' }] }],
    ['non-boolean embedding satisfied', { ...wellFormed, embeddings: [{ ...wellFormed.embeddings[0]!, satisfied: 1 }] }],
    ['non-string note where it is required', { ...wellFormed, embeddings: [{ ...wellFormed.embeddings[0]!, note: 7 }] }],
    ['missing matchedCase', { ...wellFormed, typeSwitch: [{ ...wellFormed.typeSwitch[0]!, matchedCase: undefined }] }],
    ['non-string optional note', { ...wellFormed, typeSwitch: [{ ...wellFormed.typeSwitch[0]!, note: 5 }] }],
    ['empty compile-proof line', { ...wellFormed, compileProofs: [{ line: '', pins: 'x' }] }],
    ['non-object compile proof', { ...wellFormed, compileProofs: ['nope'] }],
  ])('throws for %s', (_label, body) => {
    expect(() => interfaceMatrixDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = interfaceMatrixDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      rows: wellFormed.rows.map((row) => ({ ...row, sneaky: true })),
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toInterfaceMatrixDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toInterfaceMatrixDemo(wellFormed)).toBe(wellFormed);
  });
});
