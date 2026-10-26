import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { WebhookSimulatorRepository } from '../../domain/demo/webhook-simulator-repository';
import type { WebhookSimulatorDemoData } from '../../domain/demo/webhook-simulator';
import { toWebhookSimulatorDemo } from './webhook-simulator-dto';
import type { WebhookSimulatorApi } from './webhook-simulator-api';

/**
 * WebhookSimulatorRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class WebhookSimulatorRepositoryImpl implements WebhookSimulatorRepository {
  private readonly api: WebhookSimulatorApi;

  constructor(api: WebhookSimulatorApi) {
    this.api = api;
  }

  async get(): Promise<Result<WebhookSimulatorDemoData>> {
    return safeRepositoryCall(async () => toWebhookSimulatorDemo(await this.api.get()));
  }
}
