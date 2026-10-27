import type { AuthStoryDemoData } from '../../../domain/demo/auth-story';

/**
 * Shared fixtures for the seven auth story specs — one builder shaped
 * exactly like the server's payloads, with per-demo distinctive beats
 * so each spec pins its own demo's teaching content rather than the
 * shared chrome.
 */

export function authStoryFixture(overrides: Partial<AuthStoryDemoData> = {}): AuthStoryDemoData {
  return {
    mechanism: 'sessions',
    beats: [
      {
        beat: 'login',
        detail: 'password verified; the cookie carries only the SIGNED id.',
        verdict: 'ok',
        facts: [
          { label: 'HttpOnly', value: 'true — page JavaScript can never read it' },
          { label: 'SameSite', value: 'lax — the cookie does not ride cross-site posts' },
        ],
      },
      {
        beat: 'tampered cookie',
        detail: 'one flipped character → refused: the HMAC seal fails before the store is ever consulted.',
        verdict: 'rejected',
        facts: [],
      },
    ],
    proofs: [
      { label: 'tamper-proof', value: 'sealed cookie refused after a one-character edit: true' },
      { label: 'revocation', value: 'logout deleted the row and the same value is dead afterwards: true' },
    ],
    reasoning: 'the browser holds a tamper-evident handle, the store holds the truth.',
    ...overrides,
  };
}
