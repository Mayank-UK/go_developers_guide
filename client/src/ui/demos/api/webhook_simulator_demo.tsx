import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetWebhookSimulatorDemoUseCase,
  WEBHOOK_SIMULATOR_DEMO_USE_CASE,
} from '../../../domain/demo/get-webhook-simulator-demo-usecase';
import type {
  DeliveryLedger,
  SignatureCase,
  WebhookSimulatorDemoData,
} from '../../../domain/demo/webhook-simulator';

/**
 * WebhookSimulatorDemo — the webhooks topic's live demo (demo block id
 * `api:webhook-simulator`). It renders the payload of
 * GET /api/demo/api/webhook-simulator: a delivery the server REALLY
 * signed with HMAC-SHA256 and the receiver's gate REALLY judged — one
 * verified verdict, three refusals (tampered body, stale timestamp,
 * wrong key); two endpoints walked through the retry ladder as computed
 * data (0/1/2s recovers on attempt 3; 0/1/2/4/8s dead-letters after 5);
 * and the receiving side's front door — verify first, answer 2xx fast,
 * process async.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load.
 */

type WebhookSimulatorStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: WebhookSimulatorDemoData }
  | { state: 'error'; failure: Failure };

export function useWebhookSimulatorDemo(
  useCase: GetWebhookSimulatorDemoUseCase = container.resolve(WEBHOOK_SIMULATOR_DEMO_USE_CASE),
): { status: WebhookSimulatorStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<WebhookSimulatorStatus>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });

    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus({ state: 'ready', data: result.value });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function WebhookSimulatorDemo() {
  const { status, retry } = useWebhookSimulatorDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="webhook-simulator-loading" className="text-body-sm text-on-surface-variant">
        signing, verifying, retrying…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the webhook simulator" failure={status.failure} onRetry={retry} />;
  }

  const { config, signing, deliveries, receiver } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section
        data-testid="whs-signing"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">
          One signed delivery, judged four times
        </h4>
        <p className="font-mono text-caption text-on-surface-variant">
          {config.algorithm} over {config.messageFormat} · header {config.signatureHeader} ·
          freshness ±{config.toleranceSeconds}s
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          signed at {signing.issued.timestamp} · nonce {signing.issued.nonce} ·{' '}
          {signing.issued.signature.slice(0, 16)}…
        </p>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">case</th>
                <th scope="col" className="px-sm py-xxs">verdict</th>
                <th scope="col" className="px-sm py-xxs">the gate's reason</th>
              </tr>
            </thead>
            <tbody>
              {signing.cases.map((signatureCase) => (
                <SignatureCaseCells key={signatureCase.id} signatureCase={signatureCase} />
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          Every verdict above is the real gate's output — the refusals carry the
          check that fired, in the order the checks run: clock first, then the
          constant-time HMAC comparison.
        </p>
      </section>

      <section data-testid="whs-deliveries" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          The retry ladder: {config.delayPattern}
        </h4>
        {deliveries.map((ledger) => (
          <DeliveryLedgerCard key={ledger.deliveryId} ledger={ledger} />
        ))}
      </section>

      <section
        data-testid="whs-receiver"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">
          The receiver's front door
        </h4>
        <ol className="flex flex-col gap-xxs">
          {[...receiver.asyncBefore2xx, ...receiver.asyncAfter2xx].map((step) => (
            <li key={step.step} className="text-body-sm text-on-surface-variant">
              <span className="font-mono text-on-surface">{step.step}.</span>{' '}
              <span className="text-on-surface">{step.title}</span> — {step.detail}
            </li>
          ))}
        </ol>
        <p className="font-mono text-caption text-secondary">
          answered {receiver.answeredStatus} {receiver.answeredBody} — before any business work
        </p>
      </section>
    </div>
  );
}

function SignatureCaseCells({ signatureCase }: { signatureCase: SignatureCase }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{signatureCase.id}</td>
      <td className={`px-sm py-xxs ${signatureCase.verified ? 'text-secondary' : 'text-error'}`}>
        {signatureCase.verified ? 'verified' : 'rejected'}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">
        {signatureCase.verified ? signatureCase.note : signatureCase.reason}
      </td>
    </tr>
  );
}

function DeliveryLedgerCard({ ledger }: { ledger: DeliveryLedger }) {
  const recovered = ledger.outcome === 'delivered';
  return (
    <div data-testid={`whs-ledger-${ledger.deliveryId}`} className="flex flex-col gap-xs">
      <p className="font-mono text-caption text-on-surface">
        {ledger.deliveryId} · {ledger.event} → {ledger.endpoint}
      </p>
      <div className="overflow-x-auto rounded-md border border-outline">
        <table className="w-full text-left font-mono text-caption">
          <thead className="bg-surface-tint text-on-surface-variant">
            <tr>
              <th scope="col" className="px-sm py-xxs">attempt</th>
              <th scope="col" className="px-sm py-xxs">waited</th>
              <th scope="col" className="px-sm py-xxs">status</th>
              <th scope="col" className="px-sm py-xxs">ruling</th>
            </tr>
          </thead>
          <tbody>
            {ledger.attempts.map((row) => (
              <tr key={row.attempt} className="border-t border-outline">
                <td className="px-sm py-xxs text-on-surface">{row.attempt}</td>
                <td className="px-sm py-xxs text-on-surface-variant">
                  {row.waitSeconds === 0 ? '—' : `${row.waitSeconds}s`}
                </td>
                <td className={`px-sm py-xxs ${row.status >= 200 && row.status < 300 ? 'text-secondary' : 'text-error'}`}>
                  {row.status}
                </td>
                <td className="px-sm py-xxs text-on-surface-variant">{row.ruling}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className={`text-body-sm ${recovered ? 'text-on-surface-variant' : 'text-error'}`}>
        {recovered
          ? `delivered on attempt ${ledger.attemptsUsed} after ${ledger.totalWaitSeconds}s of computed backoff`
          : `dead-lettered after ${ledger.attemptsUsed} attempts and ${ledger.totalWaitSeconds}s — ${ledger.deadLetter?.queue}, replayable: the door out`}
      </p>
    </div>
  );
}

export default WebhookSimulatorDemo;
