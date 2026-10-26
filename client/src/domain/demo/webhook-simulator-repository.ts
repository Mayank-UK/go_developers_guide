import type { Result } from '../../core/result';
import type { WebhookSimulatorDemoData } from './webhook-simulator';

/**
 * WebhookSimulatorRepository — the domain-facing port for the webhook
 * delivery-simulator payload. Demo components depend on this interface
 * only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface WebhookSimulatorRepository {
  /** The deterministic simulation: signature verdicts, retry ledgers, receiver. */
  get(): Promise<Result<WebhookSimulatorDemoData>>;
}
