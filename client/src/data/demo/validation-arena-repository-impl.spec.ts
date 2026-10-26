import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ValidationArenaApi } from './validation-arena-api';
import { ValidationArenaRepositoryImpl } from './validation-arena-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ValidationArenaRepositoryImpl spec — fixture-driven (happy + failure):
 * the happy payload round-trips into the domain shape; network /
 * envelope / malformed-payload failures all resolve as Failed Results,
 * never throws. The fixture mirrors the Go builder's real output —
 * including the exhibit request id the server stamps on every 422.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
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
    { id: 'decode', title: 'Layer one — decode discipline', tool: 'encoding/json + net/http', summary: 'Shape and types.' },
    { id: 'rules', title: 'Layer two — hand-rolled field rules', tool: 'plain Go functions', summary: 'The backbone.' },
    { id: 'library', title: 'Layer three — the tag ecosystem', tool: 'go-playground/validator (by comparison)', summary: 'Tags as data.' },
  ],
  candidates: [
    {
      id: 'valid-complete',
      label: 'A well-formed submission',
      expects: 'The baseline every other row deviates from.',
      body: '{"title":"Loved the goroutines topic","body":"-race finally clicked.","score":5,"email":"reader@example.com"}',
      decode: { status: 'accepted' },
      validate: { status: 'validated' },
      response: {
        status: 200,
        validated: { title: 'Loved the goroutines topic', body: '-race finally clicked.', score: 5, email: 'reader@example.com' },
      },
    },
    {
      id: 'collect-every-issue',
      label: 'Three broken fields at once',
      expects: 'One submission, every fixable field reported in one 422.',
      body: '{"title":"","body":"","score":9,"email":"nope"}',
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
      { field: 'score', tag: 'required,gte=1,lte=5', libraryRule: 'numeric range', handRolled: 'whole number 1–5' },
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

function makeRepository(): ValidationArenaRepositoryImpl {
  return new ValidationArenaRepositoryImpl(new ValidationArenaApi(createAppConfig()));
}

describe('ValidationArenaRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/validation/arena', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.subject.name).toBe('FeedbackForm');
      expect(result.value.layers).toHaveLength(3);
      expect(result.value.candidates[1]?.validate.issues).toHaveLength(3);
      expect(result.value.candidates[1]?.response.envelope?.error).toEqual(
        expect.objectContaining({ code: 'validation_failed', requestId: 'demo-validation-arena' }),
      );
      expect(result.value.envelope.anatomy).toHaveLength(5);
      expect(result.value.comparison.imported).toBe(false);
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
        url: '/api/demo/validation/arena',
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
      // Missing the candidate roster — the DTO validator must reject it.
      { url: '/api/demo/validation/arena', body: { subject: demoPayload.subject } },
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

  it('rejects an unknown decode vocabulary instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/validation/arena',
        body: {
          ...demoPayload,
          candidates: [{ ...demoPayload.candidates[0], decode: { status: 'coerced' } }],
        },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });
});
