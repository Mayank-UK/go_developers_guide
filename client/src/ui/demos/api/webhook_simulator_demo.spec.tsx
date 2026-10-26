import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { WebhookSimulatorDemo } from './webhook_simulator_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetWebhookSimulatorDemoUseCase,
  WEBHOOK_SIMULATOR_DEMO_USE_CASE,
} from '../../../domain/demo/get-webhook-simulator-demo-usecase';
import type { WebhookSimulatorDemoData } from '../../../domain/demo/webhook-simulator';

/**
 * WebhookSimulatorDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the four verification verdicts render with the real gate's reasons, the
 * two retry ledgers show their computed waits and endings (one delivered,
 * one dead-lettered with its DLQ record), the receiver's five steps
 * render in order with the immediate 2xx, and failures degrade to the
 * shared error panel with recovery.
 *
 * (The demo-registry binding test lands with the composition-root wiring —
 * this slice ships before demo-registry.tsx grows its lazy entry.)
 */

const signature = '7f3a9c2b5e41d80aa6c14f2e77b9c03d55e8a1f0b2d4c6e8901a3b5c7d9e0f2a';

const fixture: WebhookSimulatorDemoData = {
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
        note: 'verified: rebuilt HMAC equals the presented one',
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
      {
        id: 'stale-timestamp',
        description: 'a seven-minute-old replay',
        receiverClock: '2026-10-26T09:07:00Z',
        presented: 'v1=7f3a9c2b5e41d80a…',
        verified: false,
        reason: 'timestamp outside the tolerance window: replay or badly skewed clock',
        note: 'rejected at the clock check',
      },
      {
        id: 'wrong-key',
        description: 'signed with a different secret',
        receiverClock: '2026-10-26T09:00:41Z',
        presented: 'v1=0000000000000000…',
        verified: false,
        reason: 'signature mismatch: wrong key or altered payload',
        note: 'indistinguishable from tampering',
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
      { step: 1, title: 'Read the raw body', detail: 'io.ReadAll — no parsing yet.' },
      { step: 2, title: 'Verify before trusting', detail: 'the gate, for real.' },
    ],
    asyncAfter2xx: [
      { step: 3, title: 'Answer 2xx immediately', detail: 'the sender stops retrying.' },
      { step: 4, title: 'Hand off to a worker', detail: 'the handler is done.' },
      { step: 5, title: 'Dedupe by eventId', detail: 'duplicates become no-ops.' },
    ],
  },
};

function stubUseCase(execute: () => Promise<Result<WebhookSimulatorDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    WEBHOOK_SIMULATOR_DEMO_USE_CASE,
    { execute } as unknown as GetWebhookSimulatorDemoUseCase,
  );
}

describe('WebhookSimulatorDemo', () => {
  it('renders loading first, then the whole simulation', async () => {
    stubUseCase(async () => success(fixture));

    render(<WebhookSimulatorDemo />);

    expect(screen.getByTestId('webhook-simulator-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('whs-signing')).toBeInTheDocument());

    // The signing facts: algorithm, canonical message, tolerance window.
    const signing = screen.getByTestId('whs-signing');
    expect(signing).toHaveTextContent('HMAC-SHA256');
    expect(signing).toHaveTextContent('{timestamp}.{nonce}.{body}');
    expect(signing).toHaveTextContent('±300s');
    expect(signing).toHaveTextContent('n-4f2c1b');

    // Four verdicts: one verified, three rejected with the gate's reasons.
    expect(signing).toHaveTextContent('verified');
    expect(signing).toHaveTextContent('rejected');
    expect(signing).toHaveTextContent('signature mismatch: wrong key or altered payload');
    expect(signing).toHaveTextContent('timestamp outside the tolerance window');

    // The two ledgers and their endings.
    const recovered = screen.getByTestId('whs-ledger-dlv_0207');
    expect(recovered).toHaveTextContent('delivered on attempt 3 after 3s of computed backoff');
    expect(recovered).toHaveTextContent('200');
    const deadLettered = screen.getByTestId('whs-ledger-dlv_0208');
    expect(deadLettered).toHaveTextContent('dead-lettered after 5 attempts and 15s');
    expect(deadLettered).toHaveTextContent('webhook-dlq');

    // The backoff waits render as data (0 renders as the em dash).
    expect(recovered).toHaveTextContent('2s');
    expect(deadLettered).toHaveTextContent('8s');

    // The receiver's front door: five steps, then the immediate 2xx.
    const receiver = screen.getByTestId('whs-receiver');
    expect(receiver).toHaveTextContent('Read the raw body');
    expect(receiver).toHaveTextContent('Dedupe by eventId');
    expect(receiver).toHaveTextContent('answered 200 {"received":true}');
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

    render(<WebhookSimulatorDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the simulation renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('whs-signing')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
