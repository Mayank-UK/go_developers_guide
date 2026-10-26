import type { Result } from '../../core/result';
import type { WebhookSimulatorRepository } from './webhook-simulator-repository';
import type { WebhookSimulatorDemoData } from './webhook-simulator';

/**
 * WEBHOOK_SIMULATOR_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(WEBHOOK_SIMULATOR_DEMO_USE_CASE, new GetWebhookSimulatorDemoUseCase(repository));
 */
export const WEBHOOK_SIMULATOR_DEMO_USE_CASE = Symbol('GetWebhookSimulatorDemoUseCase');

/**
 * GetWebhookSimulatorDemoUseCase — loads the webhooks topic's delivery
 * simulator payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the simulation's exhibit facts are all server-chosen.
 */
export class GetWebhookSimulatorDemoUseCase {
  private readonly repository: WebhookSimulatorRepository;

  constructor(repository: WebhookSimulatorRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<WebhookSimulatorDemoData>> {
    return this.repository.get();
  }
}
