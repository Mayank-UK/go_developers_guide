import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { SecurityStoryApi } from './security-story-api';
import { SecurityStoryRepositoryImpl } from './security-story-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * SecurityStoryRepositoryImpl spec — fixture-driven (happy + failure):
 * one of the two story payloads (mechanism, beats with verdicts and
 * facts, proofs, reasoning) round-trips through the shared adapter; a
 * poisoned payload and a network failure resolve as Failed Results,
 * never throws.
 */

const storyPayload = {
  mechanism: 'security-lab',
  beats: [
    {
      beat: 'the input sweep',
      detail: 'the payload is refused by the validator before any query is built — never string-spooged.',
      verdict: 'rejected',
      facts: [{ label: "bobby'; DROP TABLE", value: 'refused at the boundary, 422' }],
    },
    {
      beat: 'the header audit',
      detail: 'the probe request comes back with both hardening headers set — observed, not assumed.',
      verdict: 'ok',
      facts: [{ label: 'response headers', value: 'Content-Security-Policy + Strict-Transport-Security present' }],
    },
    {
      beat: 'the PII scan',
      detail: 'masked columns leave the server already redacted — the client never sees raw PII.',
      verdict: 'info',
      facts: [{ label: 'masked fields', value: 'pan → **** 0042 · email → j***@example.com' }],
    },
  ],
  proofs: [{ label: 'rate limit', value: 'the lab degrades to a typed 429 under hammering' }],
  reasoning: 'the lab refuses first and explains second, so every refusal is observable.',
};

function makeRepository(): SecurityStoryRepositoryImpl {
  return new SecurityStoryRepositoryImpl(new SecurityStoryApi(createAppConfig(), '/demo/security/lab'));
}

describe('SecurityStoryRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/security/lab', body: storyPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.mechanism).toBe('security-lab');
      expect(result.value.beats).toHaveLength(3);
      expect(result.value.beats[0]?.verdict).toBe('rejected');
      expect(result.value.beats[2]?.facts[0]?.label).toBe('masked fields');
      expect(result.value.proofs[0]?.label).toBe('rate limit');
      expect(result.value.reasoning).toContain('refuses first');
    }
  });

  it('rejects a payload whose proof label is empty', async () => {
    const poisoned = {
      ...storyPayload,
      proofs: [{ ...storyPayload.proofs[0], label: '' }],
    };
    stubApiFetch([{ url: '/api/demo/security/lab', body: poisoned }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
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
});
