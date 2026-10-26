import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { WebhookSimulatorDemoData } from './webhook-simulator';
import type { WebhookSimulatorRepository } from './webhook-simulator-repository';
import { GetWebhookSimulatorDemoUseCase } from './get-webhook-simulator-demo-usecase';

/**
 * GetWebhookSimulatorDemoUseCase spec — the use case is a thin
 * pass-through seam, so the contract to pin is exactly that: whatever the
 * repository resolves (Success or Failed) arrives unchanged, and nothing
 * throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<WebhookSimulatorDemoData>): WebhookSimulatorRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: WebhookSimulatorDemoData = {
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
      signature: 'a'.repeat(64),
      headerLine: 't=2026-10-26T09:00:00Z,n=n-4f2c1b,v1=' + 'a'.repeat(64),
    },
    cases: [
      {
        id: 'correct-delivery',
        description: 'the honest delivery',
        receiverClock: '2026-10-26T09:00:41Z',
        presented: 'v1=aaaaaaaaaaaaaaaa…',
        verified: true,
        reason: '',
        note: 'verified',
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
  ],
  receiver: {
    answeredStatus: 200,
    answeredBody: '{"received":true}',
    asyncBefore2xx: [
      { step: 1, title: 'Read the raw body', detail: 'io.ReadAll — no parsing yet.' },
      { step: 2, title: 'Verify before trusting', detail: 'the gate, for real.' },
    ],
    asyncAfter2xx: [
      { step: 3, title: 'Answer 2xx immediately', detail: 'the sender stops retrying.' },
    ],
  },
};

describe('GetWebhookSimulatorDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetWebhookSimulatorDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetWebhookSimulatorDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
