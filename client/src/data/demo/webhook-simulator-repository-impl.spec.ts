import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { WebhookSimulatorApi } from './webhook-simulator-api';
import { WebhookSimulatorRepositoryImpl } from './webhook-simulator-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * WebhookSimulatorRepositoryImpl spec — fixture-driven (happy + failure):
 * the happy payload round-trips into the domain shape; network /
 * envelope / malformed-payload failures all resolve as Failed Results,
 * never throws.
 */

const signature = '7f3a9c2b5e41d80aa6c14f2e77b9c03d55e8a1f0b2d4c6e8901a3b5c7d9e0f2a';

/** A structurally complete wire payload (mirrors the Go builder's output). */
const demoPayload = {
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
        description: 'the honest delivery',
        receiverClock: '2026-10-26T09:00:41Z',
        presented: 'v1=7f3a9c2b5e41d80a…',
        verified: true,
        reason: '',
        note: 'verified',
      },
      {
        id: 'tampered-payload',
        description: 'amount rewritten after signing',
        receiverClock: '2026-10-26T09:00:41Z',
        presented: 'v1=7f3a9c2b5e41d80a…',
        verified: false,
        reason: 'signature mismatch: wrong key or altered payload',
        note: 'rebuilt over the bytes that arrived',
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

function makeRepository(): WebhookSimulatorRepositoryImpl {
  return new WebhookSimulatorRepositoryImpl(new WebhookSimulatorApi(createAppConfig()));
}

describe('WebhookSimulatorRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/api/webhook-simulator', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.config.algorithm).toBe('HMAC-SHA256');
      expect(result.value.signing.cases[0]?.verified).toBe(true);
      expect(result.value.deliveries[0]?.outcome).toBe('delivered');
      expect(result.value.deliveries[1]?.deadLetter?.queue).toBe('webhook-dlq');
      expect(result.value.receiver.answeredStatus).toBe(200);
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
        url: '/api/demo/api/webhook-simulator',
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
      // Missing the retry ledgers — the DTO validator must reject it.
      { url: '/api/demo/api/webhook-simulator', body: { config: demoPayload.config } },
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

  it('rejects an unknown outcome vocabulary instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/api/webhook-simulator',
        body: {
          ...demoPayload,
          deliveries: demoPayload.deliveries.map((ledger) => ({ ...ledger, outcome: 'vanished' })),
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
