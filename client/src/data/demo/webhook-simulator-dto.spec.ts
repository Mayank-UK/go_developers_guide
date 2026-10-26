import { describe, expect, it } from 'vitest';
import { webhookSimulatorDtoFromJson, toWebhookSimulatorDemo } from './webhook-simulator-dto';
import type { WebhookSimulatorDemoData } from '../../domain/demo/webhook-simulator';

/**
 * WebhookSimulatorDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). The fixture
 * mirrors the Go builder's REAL output: one verified signature case with
 * `reason: ''` (empty is DATA — no error happened), one delivered ledger
 * whose `deadLetter` the server omits entirely, and closed vocabularies
 * (algorithm, rulings, outcomes) that reject drift.
 */

const signature = '7f3a9c2b5e41d80aa6c14f2e77b9c03d55e8a1f0b2d4c6e8901a3b5c7d9e0f2a';

const wellFormed: WebhookSimulatorDemoData = {
  config: {
    algorithm: 'HMAC-SHA256',
    signatureHeader: 'X-Guide-Signature',
    headerFormat: 't={timestamp},n={nonce},v1={hex-signature}',
    messageFormat: '{timestamp}.{nonce}.{body}',
    toleranceSeconds: 300,
    maxAttempts: 5,
    baseDelaySeconds: 1,
    maxDelaySeconds: 30,
    delayPattern: 'base × 2^(attempt-1), capped at max',
  },
  signing: {
    secretHint: 'whsec_… (exhibit key, demo-only; real keys come from the environment)',
    body: '{"eventId":"evt_0142","type":"invoice.paid","version":"1","occurredAt":"2026-10-26T08:59:59Z","data":{"invoiceId":"in_88","amountCents":42000,"currency":"USD"}}',
    issued: {
      timestamp: '2026-10-26T09:00:00Z',
      nonce: 'n-4f2c1b',
      signature,
      headerLine: `t=2026-10-26T09:00:00Z,n=n-4f2c1b,v1=${signature}`,
    },
    cases: [
      {
        id: 'correct-delivery',
        description: 'the honest delivery: original body, original signature',
        receiverClock: '2026-10-26T09:00:41Z',
        presented: 'v1=7f3a9c2b5e41d80a…',
        verified: true,
        reason: '',
        note: 'verified: rebuilt HMAC equals the presented one',
      },
      {
        id: 'tampered-payload',
        description: 'amount rewritten after signing',
        receiverClock: '2026-10-26T09:00:41Z',
        presented: 'v1=7f3a9c2b5e41d80a…',
        verified: false,
        reason: 'signature mismatch: wrong key or altered payload',
        note: 'the gate rebuilds over the bytes that ARRIVED',
      },
    ],
  },
  deliveries: [
    {
      deliveryId: 'dlv_0207',
      event: 'invoice.paid',
      endpoint: 'https://hooks.example.com/invoices',
      attempts: [
        { attempt: 1, waitSeconds: 0, status: 500, ruling: 'retry-scheduled' },
        { attempt: 2, waitSeconds: 1, status: 503, ruling: 'retry-scheduled' },
        { attempt: 3, waitSeconds: 2, status: 200, ruling: 'delivered' },
      ],
      outcome: 'delivered',
      attemptsUsed: 3,
      totalWaitSeconds: 3,
    },
    {
      deliveryId: 'dlv_0208',
      event: 'subscription.canceled',
      endpoint: 'https://legacy.example.com/hooks',
      attempts: [
        { attempt: 1, waitSeconds: 0, status: 500, ruling: 'retry-scheduled' },
        { attempt: 2, waitSeconds: 1, status: 502, ruling: 'retry-scheduled' },
        { attempt: 3, waitSeconds: 2, status: 500, ruling: 'retry-scheduled' },
        { attempt: 4, waitSeconds: 4, status: 504, ruling: 'retry-scheduled' },
        { attempt: 5, waitSeconds: 8, status: 500, ruling: 'dead-lettered' },
      ],
      outcome: 'dead-lettered',
      attemptsUsed: 5,
      totalWaitSeconds: 15,
      deadLetter: { queue: 'webhook-dlq', reason: 'no 2xx after 5 attempts', replayable: true },
    },
  ],
  receiver: {
    answeredStatus: 200,
    answeredBody: '{"received":true}',
    asyncBefore2xx: [
      { step: 1, title: 'Read the raw body', detail: 'io.ReadAll on the request stream.' },
      { step: 2, title: 'Verify before trusting', detail: 'the gate, on the exhibit delivery.' },
    ],
    asyncAfter2xx: [
      { step: 3, title: 'Answer 2xx immediately', detail: 'the sender stops retrying.' },
    ],
  },
};

describe('webhookSimulatorDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(webhookSimulatorDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps the verified case’s empty reason as data (no error happened)', () => {
    const parsed = webhookSimulatorDtoFromJson(wellFormed);

    expect(parsed.signing.cases[0]?.reason).toBe('');
    expect(parsed.signing.cases[0]?.verified).toBe(true);
  });

  it('keeps a delivered ledger’s deadLetter absent (omitted, never null)', () => {
    const parsed = webhookSimulatorDtoFromJson(wellFormed);

    expect(parsed.deliveries[0]?.deadLetter).toBeUndefined();
    expect(parsed.deliveries[1]?.deadLetter?.replayable).toBe(true);
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing config', { ...wellFormed, config: undefined }],
    ['unknown algorithm', { ...wellFormed, config: { ...wellFormed.config, algorithm: 'MD5' } }],
    ['missing signing', { ...wellFormed, signing: undefined }],
    ['missing issued signature', { ...wellFormed, signing: { ...wellFormed.signing, issued: undefined } }],
    ['missing cases', { ...wellFormed, signing: { ...wellFormed.signing, cases: undefined } }],
    ['a non-object signature case', { ...wellFormed, signing: { ...wellFormed.signing, cases: ['nope'] } }],
    ['missing verified flag', { ...wellFormed, signing: { ...wellFormed.signing, cases: [{ ...wellFormed.signing.cases[0]!, verified: undefined }] } }],
    ['null reason (null is not omission)', { ...wellFormed, signing: { ...wellFormed.signing, cases: [{ ...wellFormed.signing.cases[0]!, reason: null }] } }],
    ['missing deliveries', { ...wellFormed, deliveries: undefined }],
    ['a non-object ledger', { ...wellFormed, deliveries: ['nope'] }],
    ['unknown delivery outcome', { ...wellFormed, deliveries: [{ ...wellFormed.deliveries[0]!, outcome: 'lost' }] }],
    ['unknown attempt ruling', { ...wellFormed, deliveries: [{ ...wellFormed.deliveries[0]!, attempts: [{ ...wellFormed.deliveries[0]!.attempts[0]!, ruling: 'maybe' }] }] }],
    ['missing attempt wait', { ...wellFormed, deliveries: [{ ...wellFormed.deliveries[0]!, attempts: [{ ...wellFormed.deliveries[0]!.attempts[0]!, waitSeconds: undefined }] }] }],
    ['null deadLetter (null is not omission)', { ...wellFormed, deliveries: [{ ...wellFormed.deliveries[0]!, deadLetter: null }] }],
    ['a non-replayable boolean deadLetter', { ...wellFormed, deliveries: [{ ...wellFormed.deliveries[1]!, deadLetter: { ...wellFormed.deliveries[1]!.deadLetter!, replayable: 'yes' } }] }],
    ['missing receiver', { ...wellFormed, receiver: undefined }],
    ['missing the answered status', { ...wellFormed, receiver: { ...wellFormed.receiver, answeredStatus: undefined } }],
    ['a non-object receiver step', { ...wellFormed, receiver: { ...wellFormed.receiver, asyncAfter2xx: ['nope'] } }],
  ])('throws for %s', (_label, body) => {
    expect(() => webhookSimulatorDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = webhookSimulatorDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      deliveries: wellFormed.deliveries.map((ledger) => ({ ...ledger, sneaky: true })),
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toWebhookSimulatorDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toWebhookSimulatorDemo(wellFormed)).toBe(wellFormed);
  });
});
