import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { AuthStoryApi } from './auth-story-api';
import { AuthStoryRepositoryImpl } from './auth-story-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * AuthStoryRepositoryImpl spec — fixture-driven (happy + failure): one
 * of the seven story payloads (mechanism, beats with verdicts and
 * facts, proofs, reasoning) round-trips through the shared adapter; a
 * poisoned payload and a network failure resolve as Failed Results,
 * never throws.
 */

const storyPayload = {
  mechanism: 'auth-jwt',
  beats: [
    {
      beat: 'the login',
      detail: 'credentials verified against the stored argon2id hash, a session pair minted.',
      verdict: 'ok',
      facts: [{ label: 'Set-Cookie', value: '__Host-session=…; Secure; HttpOnly; SameSite=Lax' }],
    },
    {
      beat: 'the tampered token',
      detail: 'one byte of payload edited — the signature no longer verifies, the request dies.',
      verdict: 'rejected',
      facts: [{ label: 'verify error', value: 'signature is invalid after the payload edit' }],
    },
    {
      beat: 'the algorithm strip',
      detail: 'alg removed from the header: the server refuses to guess and treats it as RS256.',
      verdict: 'alarm',
      facts: [{ label: 'alg', value: 'absent → refused, never defaulted' }],
    },
  ],
  proofs: [{ label: 'token lifetime', value: '15m access · 30d refresh, rotated on use' }],
  reasoning: 'every check the server performs is a check the client can watch fail.',
};

function makeRepository(): AuthStoryRepositoryImpl {
  return new AuthStoryRepositoryImpl(new AuthStoryApi(createAppConfig(), '/demo/auth/jwt'));
}

describe('AuthStoryRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/auth/jwt', body: storyPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.mechanism).toBe('auth-jwt');
      expect(result.value.beats).toHaveLength(3);
      expect(result.value.beats[1]?.verdict).toBe('rejected');
      expect(result.value.beats[2]?.facts[0]?.label).toBe('alg');
      expect(result.value.proofs[0]?.label).toBe('token lifetime');
      expect(result.value.reasoning).toContain('watch fail');
    }
  });

  it('rejects a payload whose beat verdict is empty', async () => {
    const poisoned = {
      ...storyPayload,
      beats: [{ ...storyPayload.beats[0], verdict: '' }],
    };
    stubApiFetch([{ url: '/api/demo/auth/jwt', body: poisoned }]);

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
