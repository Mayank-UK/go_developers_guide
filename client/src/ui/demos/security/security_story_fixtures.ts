import type { SecurityStoryDemoData } from '../../../domain/demo/security-story';

/**
 * Shared fixtures for the two security story specs — builders shaped
 * exactly like the server's payloads, with per-demo distinctive beats
 * so each spec pins its own demo's teaching content rather than the
 * shared chrome.
 */

/** A lab-shaped story: the header set, the burst, the 429 boundary. */
export function securityLabFixture(
  overrides: Partial<SecurityStoryDemoData> = {},
): SecurityStoryDemoData {
  return {
    mechanism: 'security-lab',
    beats: [
      {
        beat: 'the header set',
        detail:
          'every response this route serves carries the 5-header policy below — set BEFORE the handler runs.',
        verdict: 'ok',
        facts: [
          { label: 'Content-Security-Policy', value: "default-src 'none'; frame-ancestors 'none'" },
          { label: 'X-Content-Type-Options', value: 'nosniff' },
        ],
      },
      {
        beat: 'the 429 boundary',
        detail: 'request #6 finds the bucket empty → 429 rate_limited, retry after 90ms.',
        verdict: 'rejected',
        facts: [{ label: 'the window', value: '#1=200 #2=200 #3=200 #4=200 #5=200 #6=429' }],
      },
    ],
    proofs: [
      {
        label: 'burst vs sustained',
        value: '10/s sustained, burst 5 — burst is the instant budget, rate is the refill speed',
      },
    ],
    reasoning: 'the headers are the response’s own armor; the window refills continuously.',
    ...overrides,
  };
}

/** A PII-shaped story: classification, masking, and the clean audit. */
export function securityPiiFixture(
  overrides: Partial<SecurityStoryDemoData> = {},
): SecurityStoryDemoData {
  return {
    mechanism: 'security-pii',
    beats: [
      {
        beat: 'classification',
        detail: 'one fixture record for sub_ada, every field stamped at definition time.',
        verdict: 'info',
        facts: [
          { label: 'email', value: 'pii — ada@example.com' },
          { label: 'signup_ip', value: 'pii — 198.51.100.7' },
        ],
      },
      {
        beat: 'the audit',
        detail: 'the audit swept the store, BOTH logged reads, and the post-erasure view: absent everywhere.',
        verdict: 'ok',
        facts: [{ label: 'log', value: 'subject=sub_ada action=erase record=erased' }],
      },
    ],
    proofs: [
      { label: 'the log cannot leak', value: 'AccessLogEntry holds subject id + action + markers' },
    ],
    reasoning: 'classify at definition, minimize in serving, log access without values, erase completely.',
    ...overrides,
  };
}
