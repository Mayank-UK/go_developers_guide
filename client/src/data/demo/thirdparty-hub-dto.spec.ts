import { describe, expect, it } from 'vitest';
import { thirdPartyHubDtoFromJson } from './thirdparty-hub-dto';

/**
 * ThirdPartyHubDTO spec — the fromJson whitelist: a full hub payload
 * builds the domain shape (wiring panel, the three vendor panels with
 * their sandbox rows and counts, the taxonomy map), a SUCCESS row's
 * absent optional fields decode as undefined (omitempty on the wire —
  * empty is DATA for the strings that can be empty), and every
 * violation (missing panels, wrong types, non-boolean retryable)
 * throws so parseDto can upgrade it to an InvalidResponseException.
 */

const fullPayload = {
  mode: 'adapter-surface hub: three vendor ports, each with two real Go implementations',
  honestyNote: 'No-invention pin: every count on this payload was counted during the run.',
  wiring: {
    source: 'GET/PATCH /api/demo/scenario — the dataMode field',
    current: 'fake',
    getEndpoint: '/api/demo/scenario',
    patchEndpoint: '/api/demo/scenario',
    rows: [
      {
        category: 'mailer',
        port: 'ThirdPartyMailer',
        fakeAdapter: 'thirdPartySandboxMailer — greylisting, quota, signed acceptances',
        liveAdapter: 'thirdPartyLiveMailer — credential-gated skeleton',
        active: 'fakeAdapter',
        activeOutcome:
          'greylisted first, accepted on the retry: msg_sandbox_01 — the sandbox’s own protocol',
      },
    ],
    note: 'the same runtime swap the posts surface performs',
  },
  vendors: [
    {
      id: 'mailer',
      title: 'Email: the mailer port',
      lead: 'One Send method, one receipt, everything relay-shaped hidden behind it.',
      port: {
        name: 'ThirdPartyMailer',
        methods: [
          {
            name: 'Send',
            signature: 'Send(ctx context.Context, draft ThirdPartyMailDraft) (ThirdPartySendReceipt, error)',
          },
        ],
        implementations: [
          'thirdPartySandboxMailer — compile-time asserted (var _ ThirdPartyMailer)',
          'thirdPartyLiveMailer — compile-time asserted (var _ ThirdPartyMailer)',
        ],
        note: 'the port carries the message, not the transport',
      },
      sandbox: {
        config: ['send budget: 3 accepted sends per window'],
        rows: [
          {
            step: 1,
            call: 'Send',
            input: 'To: "not-an-address"',
            outcome: 'refused-invalid',
            vendorCode: 'invalid_recipient',
            vendorStatus: 422,
            kind: 'validation_failed',
            httpStatus: 422,
            retryable: false,
            detail: 'recipient "not-an-address" is not a bare address (local part @ domain)',
          },
          {
            step: 3,
            call: 'Send (the retry)',
            input: 'To: ops@example.com — same message, retried',
            outcome: 'accepted-after-retry',
            retryable: false,
            detail: 'accepted as msg_sandbox_01, signature v1=abc123',
          },
        ],
        counts: [{ label: 'messages accepted', value: 3 }],
      },
      live: {
        vendor: 'an email relay',
        apiBaseUrl: 'https://api.mail-relay.example',
        keyEnvVar: 'MAIL_RELAY_API_KEY',
        callSite: 'POST /v1/email/send',
        outcome: {
          step: 1,
          call: 'Send',
          input: 'MAIL_RELAY_API_KEY (by name — the value never crosses this wire)',
          outcome: 'refused-honestly',
          vendorCode: 'credentials_absent',
          vendorStatus: 503,
          kind: 'service_unavailable',
          httpStatus: 503,
          retryable: false,
          detail: 'MAIL_RELAY_API_KEY is not set in this environment',
        },
        note: 'the skeleton resolves the key by name and refuses honestly',
      },
    },
  ],
  taxonomyMap: [
    {
      vendorCode: 'card_declined',
      vendorStatus: 402,
      kind: 'validation_failed',
      httpStatus: 422,
      retryable: false,
      viaRawStatus: false,
      note: 'a decline is input-shaped (the vendor’s 402 is NOT our 402)',
    },
    {
      vendorCode: 'no_such_object',
      vendorStatus: 404,
      kind: 'not_found',
      httpStatus: 404,
      retryable: false,
      viaRawStatus: true,
      note: 'raw-status path: the real core.KindForHTTPStatus(404) answers not_found',
    },
  ],
};

/**
 * Checked narrows for the hostile mutators below — they reach deep into
 * the unknown-shaped fixture, so every hop is verified at runtime and a
 * broken fixture fails loudly instead of type-erasing its way through.
 */
function arrayOf(value: unknown, path: string): unknown[] {
  if (!Array.isArray(value)) throw new Error(`fixture: ${path} is not an array`);
  return value;
}

function recordOf(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null) {
    throw new Error(`fixture: ${path} is not a record`);
  }
  return value as Record<string, unknown>;
}

describe('thirdparty-hub-dto', () => {
  it('builds the domain shape from a full hub payload (whitelist)', () => {
    const demo = thirdPartyHubDtoFromJson(structuredClone(fullPayload));

    expect(demo.wiring.current).toBe('fake');
    expect(demo.wiring.rows[0]?.activeOutcome).toContain('msg_sandbox_01');

    const mailer = demo.vendors[0];
    expect(mailer?.port.methods[0]?.name).toBe('Send');
    expect(mailer?.port.implementations).toHaveLength(2);

    const refused = mailer?.sandbox.rows[0];
    expect(refused?.vendorCode).toBe('invalid_recipient');
    expect(refused?.httpStatus).toBe(422);
    // A SUCCESS row carries no vendor fields — omitempty decodes as undefined.
    const accepted = mailer?.sandbox.rows[1];
    expect(accepted?.vendorCode).toBeUndefined();
    expect(accepted?.vendorStatus).toBeUndefined();
    expect(accepted?.kind).toBeUndefined();
    expect(accepted?.httpStatus).toBeUndefined();

    expect(mailer?.sandbox.counts[0]).toEqual({ label: 'messages accepted', value: 3 });
    expect(mailer?.live.keyEnvVar).toBe('MAIL_RELAY_API_KEY');
    expect(mailer?.live.outcome.outcome).toBe('refused-honestly');

    expect(demo.taxonomyMap).toHaveLength(2);
    expect(demo.taxonomyMap[1]?.viaRawStatus).toBe(true);
  });

  it('drops unknown keys (whitelist, not pass-through)', () => {
    const padded = { ...structuredClone(fullPayload), surprise: 'nope' } as Record<string, unknown>;
    const demo = thirdPartyHubDtoFromJson(padded) as unknown as Record<string, unknown>;
    expect(demo['surprise']).toBeUndefined();
  });

  it.each([
    ['missing wiring panel', (p: Record<string, unknown>) => delete p['wiring']],
    ['missing vendors', (p: Record<string, unknown>) => delete p['vendors']],
    ['missing taxonomy map', (p: Record<string, unknown>) => delete p['taxonomyMap']],
    [
      'non-boolean retryable',
      (p: Record<string, unknown>) => {
        const vendor = recordOf(arrayOf(p['vendors'], 'vendors')[0], 'vendors[0]');
        const sandbox = recordOf(vendor['sandbox'], 'vendors[0].sandbox');
        const rows = arrayOf(sandbox['rows'], 'vendors[0].sandbox.rows');
        recordOf(rows[0], 'sandbox row 0')['retryable'] = 'no';
      },
    ],
    [
      'wiring row without the active field',
      (p: Record<string, unknown>) => {
        const wiring = recordOf(p['wiring'], 'wiring');
        const rows = arrayOf(wiring['rows'], 'wiring.rows');
        delete recordOf(rows[0], 'wiring row 0')['active'];
      },
    ],
    [
      'live panel without its outcome row',
      (p: Record<string, unknown>) => {
        const vendor = recordOf(arrayOf(p['vendors'], 'vendors')[0], 'vendors[0]');
        const live = recordOf(vendor['live'], 'vendors[0].live');
        delete live['outcome'];
      },
    ],
  ])('throws on %s', (_name, mutate) => {
    const payload = structuredClone(fullPayload) as Record<string, unknown>;
    mutate(payload);
    expect(() => thirdPartyHubDtoFromJson(payload)).toThrow();
  });
});
