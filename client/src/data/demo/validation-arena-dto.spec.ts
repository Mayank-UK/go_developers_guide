import { describe, expect, it } from 'vitest';
import { validationArenaDtoFromJson, toValidationArenaDemo } from './validation-arena-dto';
import type { ValidationArenaDemoData } from '../../domain/demo/validation-arena';

/**
 * ValidationArenaDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). The fixture
 * mirrors the Go builder's real output (twelve candidates server-side;
 * three here cover every outcome shape: accepted, decode-refused,
 * rule-refused). Optional wire keys are OMITTED when absent — undefined
 * here, never null. An empty `body` string is DATA (the zero-value
 * default), not absence — it passes.
 */

const wellFormed: ValidationArenaDemoData = {
  subject: {
    name: 'FeedbackForm',
    source: 'server/internal/domain/demos/validation_arena.go',
    fields: [
      { name: 'title', type: 'string', rule: 'required; at most 80 runes', required: true },
      { name: 'body', type: 'string', rule: 'optional; at most 280 runes', required: false },
      { name: 'score', type: 'int', rule: 'required; whole number 1–5', required: true },
      { name: 'email', type: 'string', rule: 'required; one @ shape', required: true },
    ],
  },
  layers: [
    { id: 'decode', title: 'Layer one', tool: 'encoding/json + net/http', summary: 'Shape and types.' },
    { id: 'rules', title: 'Layer two', tool: 'plain Go functions', summary: 'The backbone.' },
    { id: 'library', title: 'Layer three', tool: 'go-playground/validator (by comparison)', summary: 'Tags as data.' },
  ],
  candidates: [
    {
      id: 'valid-complete',
      label: 'A well-formed submission',
      expects: 'The baseline.',
      body: '{"title":"Loved the goroutines topic","score":5,"email":"reader@example.com"}',
      decode: { status: 'accepted' },
      validate: { status: 'validated' },
      response: {
        status: 200,
        validated: { title: 'Loved the goroutines topic', body: '', score: 5, email: 'reader@example.com' },
      },
      note: 'No coercion anywhere.',
    },
    {
      id: 'wrong-type-score',
      label: 'A string where an int lives',
      expects: 'The declared type IS a rule.',
      body: '{"score":"five"}',
      decode: {
        status: 'rejected',
        errorType: '*json.UnmarshalTypeError',
        error: 'json: cannot unmarshal string into Go struct field FeedbackForm.score of type int',
      },
      validate: { status: 'skipped' },
      response: {
        status: 422,
        envelope: {
          error: {
            code: 'validation_failed',
            message: 'Invalid feedback body.',
            requestId: 'demo-validation-arena',
            details: { body: 'json: cannot unmarshal string' },
          },
        },
      },
    },
    {
      id: 'collect-every-issue',
      label: 'Three broken fields at once',
      expects: 'The collector discipline.',
      body: '{"title":"","score":9,"email":"nope"}',
      decode: { status: 'accepted' },
      validate: {
        status: 'rejected',
        issues: [
          { field: 'title', rule: 'required — present-but-empty is data', got: '""' },
          { field: 'score', rule: 'required, whole number 1–5', got: '9' },
          { field: 'email', rule: 'one @ with text on both sides', got: '"nope"' },
        ],
      },
      response: {
        status: 422,
        envelope: {
          error: {
            code: 'validation_failed',
            message: 'Feedback submission failed validation.',
            requestId: 'demo-validation-arena',
            details: [{ field: 'title', rule: 'required', got: '""' }],
          },
        },
      },
    },
  ],
  comparison: {
    library: 'github.com/go-playground/validator/v10',
    imported: false,
    rows: [
      { field: 'title', tag: 'required,max=80', libraryRule: 'non-zero + rune bound', handRolled: 'trimmed non-blank, 80 runes' },
      { field: 'body', tag: 'max=280', libraryRule: 'no required tag', handRolled: 'optional, 280 runes' },
    ],
    note: 'The tags are inert without a library that reads them.',
  },
  envelope: {
    candidateId: 'collect-every-issue',
    status: 422,
    body: {
      error: {
        code: 'validation_failed',
        message: 'Feedback submission failed validation.',
        requestId: 'demo-validation-arena',
        details: [{ field: 'title', rule: 'required', got: '""' }],
      },
    },
    anatomy: [
      { path: 'status line', value: '422 Unprocessable Entity', why: 'the family convention' },
      { path: 'error.code', value: 'validation_failed', why: 'the taxonomy bucket' },
      { path: 'error.requestId', value: 'demo-validation-arena', why: 'the join key' },
    ],
  },
  reasoning: 'Twelve candidate bodies, each run through the real two-layer pipeline.',
};

describe('validationArenaDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(validationArenaDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps an empty validated body as data (the zero-value default echoes as "")', () => {
    const parsed = validationArenaDtoFromJson(wellFormed);

    expect(parsed.candidates[0]?.response.validated?.body).toBe('');
  });

  it('keeps optional facts absent (omitted on the wire, undefined here)', () => {
    const minimal = {
      subject: wellFormed.subject,
      layers: wellFormed.layers,
      candidates: [
        {
          id: 'c',
          label: 'l',
          expects: 'e',
          body: '{}',
          decode: { status: 'accepted' },
          validate: { status: 'validated' },
          response: { status: 200 },
        },
      ],
      comparison: wellFormed.comparison,
      envelope: wellFormed.envelope,
      reasoning: 'r',
    };

    const parsed = validationArenaDtoFromJson(minimal);

    expect(parsed.candidates[0]?.note).toBeUndefined();
    expect(parsed.candidates[0]?.decode.errorType).toBeUndefined();
    expect(parsed.candidates[0]?.validate.issues).toBeUndefined();
    expect(parsed.candidates[0]?.response.envelope).toBeUndefined();
    expect(parsed.candidates[0]?.response.validated).toBeUndefined();
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing subject', { ...wellFormed, subject: undefined }],
    ['missing layers', { ...wellFormed, layers: undefined }],
    ['missing candidates', { ...wellFormed, candidates: undefined }],
    ['missing comparison', { ...wellFormed, comparison: undefined }],
    ['missing envelope', { ...wellFormed, envelope: undefined }],
    ['missing reasoning', { ...wellFormed, reasoning: undefined }],
    ['a non-object candidate', { ...wellFormed, candidates: ['nope'] }],
    ['an unknown decode vocabulary', { ...wellFormed, candidates: [{ ...wellFormed.candidates[0]!, decode: { status: 'coerced' } }] }],
    ['an unknown rules vocabulary', { ...wellFormed, candidates: [{ ...wellFormed.candidates[0]!, validate: { status: 'coerced' } }] }],
    ['an unknown layer id', { ...wellFormed, layers: [{ ...wellFormed.layers[0]!, id: 'cosmic' }] }],
    ['a non-number response status', { ...wellFormed, candidates: [{ ...wellFormed.candidates[0]!, response: { status: '200' } }] }],
    ['a null note (null is not omission)', { ...wellFormed, candidates: [{ ...wellFormed.candidates[0]!, note: null }] }],
    ['a null envelope on a 422 (null is not omission)', { ...wellFormed, candidates: [{ ...wellFormed.candidates[1]!, response: { status: 422, envelope: null } }] }],
    ['a non-record envelope exhibit body', { ...wellFormed, envelope: { ...wellFormed.envelope, body: 'nope' } }],
    ['a non-boolean imported flag', { ...wellFormed, comparison: { ...wellFormed.comparison, imported: 'no' } }],
    ['a non-array issues list', { ...wellFormed, candidates: [{ ...wellFormed.candidates[2]!, validate: { status: 'rejected', issues: 'nope' } }] }],
  ])('throws for %s', (_label, body) => {
    expect(() => validationArenaDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = validationArenaDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      candidates: wellFormed.candidates.map((candidate) => ({ ...candidate, sneaky: true })),
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toValidationArenaDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toValidationArenaDemo(wellFormed)).toBe(wellFormed);
  });
});
