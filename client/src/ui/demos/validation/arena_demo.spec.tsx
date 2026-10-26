import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ArenaDemo } from './arena_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetValidationArenaDemoUseCase,
  VALIDATION_ARENA_DEMO_USE_CASE,
} from '../../../domain/demo/get-validation-arena-demo-usecase';
import type { ValidationArenaDemoData } from '../../../domain/demo/validation-arena';

/**
 * ArenaDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration instead
 * of stubbing fetch. Behavior, not implementation: the judged contract
 * renders with required-ness, the layer stack renders in order,
 * candidates render per-row verdicts with their issues and decode
 * errors, 422 rows wear the refusal styling, the comparison states the
 * library is NOT imported, the dissected envelope and its anatomy
 * render, and failures degrade to the shared error panel with recovery.
 *
 * (The demo-registry binding test lands with the composition-root wiring —
 * this slice ships before demo-registry.tsx grows its lazy entry.)
 */

const fixture: ValidationArenaDemoData = {
  subject: {
    name: 'FeedbackForm',
    source: 'server/internal/domain/demos/validation_arena.go',
    fields: [
      { name: 'title', type: 'string', rule: 'required; trimmed non-blank; at most 80 runes', required: true },
      { name: 'body', type: 'string', rule: 'optional; at most 280 runes when present', required: false },
      { name: 'score', type: 'int', rule: 'required; whole number 1–5', required: true },
      { name: 'email', type: 'string', rule: 'required; one @ with non-empty text on both sides', required: true },
    ],
  },
  layers: [
    { id: 'decode', title: 'Layer one — decode discipline', tool: 'encoding/json + net/http', summary: 'Shape and types: one value, known fields, declared types.' },
    { id: 'rules', title: 'Layer two — hand-rolled field rules', tool: 'plain Go functions', summary: 'The backbone: bounds, ranges, shapes, collected issues.' },
    { id: 'library', title: 'Layer three — the tag ecosystem', tool: 'go-playground/validator (by comparison)', summary: 'Tags as data; inert without the library.' },
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
          { field: 'title', rule: 'required — present-but-empty is data, and data can be wrong', got: '""' },
          { field: 'score', rule: 'required, whole number 1–5 — absent decodes to 0 and fails the same gate', got: '9' },
          { field: 'email', rule: 'required — one @ with non-empty text on both sides (a shape check, not the RFC grammar)', got: '"nope"' },
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
      note: 'The envelope exhibit is THIS candidate and its real 422.',
    },
  ],
  comparison: {
    library: 'github.com/go-playground/validator/v10',
    imported: false,
    rows: [
      { field: 'title', tag: 'required,max=80', libraryRule: 'non-zero + rune bound', handRolled: 'trimmed non-blank, at most 80 runes' },
      { field: 'body', tag: 'max=280', libraryRule: 'no required tag, zero passes', handRolled: 'optional; at most 280 runes' },
      { field: 'score', tag: 'required,gte=1,lte=5', libraryRule: 'numeric range; required means non-zero', handRolled: 'whole number 1–5' },
      { field: 'email', tag: 'required,email', libraryRule: 'RFC-flavored grammar', handRolled: 'one-@ shape check' },
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
      { path: 'error.message', value: 'one human sentence', why: 'safe to show any user' },
      { path: 'error.requestId', value: 'demo-validation-arena', why: 'the join key for debugging' },
      { path: 'error.details', value: 'the issue array', why: 'field, rule, got — omitted when absent, never null' },
    ],
  },
  reasoning: 'Twelve candidate bodies, each run through the real two-layer pipeline.',
};

function stubUseCase(execute: () => Promise<Result<ValidationArenaDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    VALIDATION_ARENA_DEMO_USE_CASE,
    { execute } as unknown as GetValidationArenaDemoUseCase,
  );
}

describe('ArenaDemo', () => {
  it('renders loading first, then the full arena', async () => {
    stubUseCase(async () => success(fixture));

    render(<ArenaDemo />);

    expect(screen.getByTestId('validation-arena-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('va-subject')).toBeInTheDocument());

    // The judged contract: every field with its rule and required-ness.
    const subject = screen.getByTestId('va-subject');
    expect(subject).toHaveTextContent('FeedbackForm');
    expect(subject).toHaveTextContent('title');
    expect(subject).toHaveTextContent('optional');
    expect(subject).toHaveTextContent('whole number 1–5');

    // The three layers render in stack order.
    const layers = screen.getByTestId('va-layers');
    expect(layers).toHaveTextContent('Layer one — decode discipline');
    expect(layers).toHaveTextContent('Layer two — hand-rolled field rules');
    expect(layers).toHaveTextContent('Layer three — the tag ecosystem');

    // Per-candidate verdicts: the happy row, the decode refusal with its
    // real error, and the collector row with every issue.
    expect(screen.getByTestId('va-candidate-valid-complete')).toHaveTextContent('200');
    expect(screen.getByTestId('va-candidate-valid-complete')).toHaveTextContent('validated');

    const decodeRefused = screen.getByTestId('va-candidate-wrong-type-score');
    expect(decodeRefused).toHaveTextContent('422');
    expect(decodeRefused).toHaveTextContent('*json.UnmarshalTypeError');
    expect(decodeRefused).toHaveTextContent('skipped');

    const collector = screen.getByTestId('va-candidate-collect-every-issue');
    expect(collector).toHaveTextContent('present-but-empty is data');
    expect(collector).toHaveTextContent('got "nope"');

    // The comparison is honest: library named, imported false.
    const comparison = screen.getByTestId('va-comparison');
    expect(comparison).toHaveTextContent('github.com/go-playground/validator/v10');
    expect(comparison).toHaveTextContent('false');
    expect(comparison).toHaveTextContent('required,gte=1,lte=5');

    // The dissected envelope and its anatomy.
    const envelope = screen.getByTestId('va-envelope');
    expect(envelope).toHaveTextContent('validation_failed');
    expect(envelope).toHaveTextContent('demo-validation-arena');
    expect(envelope).toHaveTextContent('the join key for debugging');

    expect(screen.getByTestId('va-reasoning')).toHaveTextContent('two-layer pipeline');
  });

  it('degrades to the shared error panel on a Failed Result, with recovery', async () => {
    let calls = 0;
    stubUseCase(async () => {
      calls += 1;
      return calls === 1
        ? failed(
            new ServerFailure('The demo endpoint is down.', {
              code: 'service_unavailable',
              status: 503,
              requestId: 'req-31',
            }),
          )
        : success(fixture);
    });

    render(<ArenaDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the arena renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('va-subject')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
