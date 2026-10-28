import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ThirdPartyHubDemo } from './hub_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetThirdPartyHubDemoUseCase,
  THIRD_PARTY_HUB_DEMO_USE_CASE,
} from '../../../domain/demo/get-thirdparty-hub-demo-usecase';
import type { ThirdPartyHubDemoData } from '../../../domain/demo/thirdparty-hub';

/**
 * ThirdPartyHubDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token. Behavior,
 * not implementation: the wiring panel states the DI switch's position
 * and per-vendor rows (sandbox vs live skeleton per adapter); each
 * vendor panel renders its port, the sandbox's observed rows with every
 * failure classified onto the taxonomy, the counted totals, and the
 * live skeleton's honest refusal; the taxonomy map renders the
 * vendor-code → kind mapping with its via-raw-status and retryable
 * markers; and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: ThirdPartyHubDemoData = {
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
        activeOutcome: 'greylisted first, accepted on the retry: msg_sandbox_01 — the sandbox’s own protocol',
      },
      {
        category: 'payments',
        port: 'ThirdPartyPayments',
        fakeAdapter: 'thirdPartySandboxPayments — declines, idempotency, quota',
        liveAdapter: 'thirdPartyLivePayments — credential-gated skeleton',
        active: 'liveAdapter',
        activeOutcome: 'refused-honestly: PAYMENTS_API_KEY is not set in this environment',
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
          { name: 'Send', signature: 'Send(ctx context.Context, draft ThirdPartyMailDraft) (ThirdPartySendReceipt, error)' },
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
            step: 2,
            call: 'Send (the retry)',
            input: 'To: ops@example.com — same message, retried',
            outcome: 'accepted-after-retry',
            retryable: false,
            detail: 'accepted as msg_sandbox_01, signature v1=abc123',
          },
        ],
        counts: [{ label: 'messages accepted', value: 1 }],
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
    {
      id: 'payments',
      title: 'Money: the payments port',
      lead: 'A charge in, a receipt out — declines arrive as data, not panics.',
      port: {
        name: 'ThirdPartyPayments',
        methods: [
          { name: 'Charge', signature: 'Charge(ctx context.Context, charge ThirdPartyCharge) (ThirdPartyChargeReceipt, error)' },
        ],
        implementations: [
          'thirdPartySandboxPayments — compile-time asserted (var _ ThirdPartyPayments)',
          'thirdPartyLivePayments — compile-time asserted (var _ ThirdPartyPayments)',
        ],
        note: 'the port carries the intent, not the processor',
      },
      sandbox: {
        config: ['idempotency: identical retries answer the first receipt'],
        rows: [
          {
            step: 3,
            call: 'Charge',
            input: 'USD 48.00 · card ending 0002',
            outcome: 'rejected-declined',
            vendorCode: 'card_declined',
            vendorStatus: 402,
            kind: 'validation_failed',
            httpStatus: 422,
            retryable: false,
            detail: 'the vendor declined; a decline is input-shaped, not our 402',
          },
          {
            step: 4,
            call: 'Charge (the retry)',
            input: 'USD 12.00 · idempotency key idem_01, retried',
            outcome: 'charged',
            retryable: false,
            detail: 'charged as ch_sandbox_01 under the idempotency key',
          },
        ],
        counts: [{ label: 'charges accepted', value: 1 }],
      },
      live: {
        vendor: 'a card processor',
        apiBaseUrl: 'https://api.card-processor.example',
        keyEnvVar: 'PAYMENTS_API_KEY',
        callSite: 'POST /v1/charges',
        outcome: {
          step: 1,
          call: 'Charge',
          input: 'PAYMENTS_API_KEY (by name — the value never crosses this wire)',
          outcome: 'refused-honestly',
          vendorCode: 'credentials_absent',
          vendorStatus: 503,
          kind: 'service_unavailable',
          httpStatus: 503,
          retryable: false,
          detail: 'PAYMENTS_API_KEY is not set in this environment',
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

function stubUseCase(execute: () => Promise<Result<ThirdPartyHubDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    THIRD_PARTY_HUB_DEMO_USE_CASE,
    { execute } as unknown as GetThirdPartyHubDemoUseCase,
  );
}

describe('ThirdPartyHubDemo', () => {
  it('renders loading first, then the wiring panel, vendor panels, taxonomy map, and honesty note', async () => {
    stubUseCase(async () => success(fixture));

    render(<ThirdPartyHubDemo />);

    expect(screen.getByTestId('thirdparty-hub-loading')).toBeInTheDocument();

    await waitForReady();

    const wiring = screen.getByTestId('thirdparty-hub-wiring');
    expect(wiring).toHaveTextContent('dataMode: fake');
    expect(wiring).toHaveTextContent('GET/PATCH /api/demo/scenario');
    // Each vendor row states which adapter the switch selects.
    const mailerWiring = screen.getByTestId('tp-wiring-mailer');
    expect(mailerWiring).toHaveTextContent('ThirdPartyMailer');
    expect(mailerWiring).toHaveTextContent('sandbox');
    expect(mailerWiring).toHaveTextContent('msg_sandbox_01');
    const paymentsWiring = screen.getByTestId('tp-wiring-payments');
    expect(paymentsWiring).toHaveTextContent('live skeleton');
    expect(paymentsWiring).toHaveTextContent('PAYMENTS_API_KEY');

    // Both vendor panels render their port, sandbox run, and live refusal.
    const mailer = screen.getByTestId('tp-vendor-mailer');
    expect(mailer).toBeInTheDocument();
    const mailerPort = screen.getByTestId('tp-port-mailer');
    expect(mailerPort).toHaveTextContent('Send(ctx context.Context, draft ThirdPartyMailDraft)');
    expect(mailerPort).toHaveTextContent('var _ ThirdPartyMailer');

    const mailerSandbox = screen.getByTestId('tp-sandbox-mailer');
    expect(mailerSandbox).toHaveTextContent('send budget: 3 accepted sends per window');
    // The refused row carries its full taxonomy classification.
    expect(screen.getByTestId('tp-row-1')).toHaveTextContent('refused-invalid');
    expect(mailerSandbox).toHaveTextContent('invalid_recipient (vendor 422) → validation_failed 422 · permanent');
    expect(screen.getByTestId('tp-row-2')).toHaveTextContent('accepted-after-retry');
    expect(screen.getByTestId('tp-row-3')).toHaveTextContent('rejected-declined');
    expect(screen.getByTestId('tp-count-mailer')).toHaveTextContent('1× messages accepted');

    const mailerLive = screen.getByTestId('tp-live-mailer');
    expect(mailerLive).toHaveTextContent('MAIL_RELAY_API_KEY');
    expect(mailerLive).toHaveTextContent('refused-honestly (service_unavailable 503)');

    const payments = screen.getByTestId('tp-vendor-payments');
    expect(payments).toBeInTheDocument();
    const paymentsSandbox = screen.getByTestId('tp-sandbox-payments');
    expect(paymentsSandbox).toHaveTextContent('card_declined (vendor 402) → validation_failed 422 · permanent');
    expect(paymentsSandbox).toHaveTextContent('charged as ch_sandbox_01');
    expect(screen.getByTestId('tp-count-payments')).toHaveTextContent('1× charges accepted');

    const taxonomy = screen.getByTestId('thirdparty-hub-taxonomy');
    expect(taxonomy).toHaveTextContent('The vendor-error taxonomy map (2 codes)');
    expect(taxonomy).toHaveTextContent('card_declined (vendor 402) → validation_failed 422');
    expect(taxonomy).toHaveTextContent('no_such_object (vendor 404) → not_found 404');
    expect(taxonomy).toHaveTextContent('via raw status');
    expect(taxonomy).toHaveTextContent('permanent');

    expect(screen.getByTestId('thirdparty-hub-honesty')).toHaveTextContent('counted during the run');
    expect(screen.getByTestId('thirdparty-hub-replay')).toHaveTextContent('replay the hub');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure(
      'The third-party hub could not observe its vendor adapters: the hub endpoint answered 503.',
      { code: 'service_unavailable', status: 503, requestId: 'rid-tp-hub' },
    );
    stubUseCase(async () => failed(failure));

    render(<ThirdPartyHubDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('the third-party hub');
    expect(panel).toHaveTextContent('service_unavailable · status 503');

    // Retry re-serves the deterministic hub through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('thirdparty-hub-ready')).toBeInTheDocument());
}
