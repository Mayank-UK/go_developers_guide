import type { Result } from '../../core/result';
import type { ThirdPartyHubRepository } from './thirdparty-hub-repository';
import type { ThirdPartyHubDemoData } from './thirdparty-hub';

/**
 * THIRD_PARTY_HUB_DEMO_USE_CASE — the container token this demo's use
 * case is registered under (same aliasing story as every demo's
 * token); the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(THIRD_PARTY_HUB_DEMO_USE_CASE, new GetThirdPartyHubDemoUseCase(repository));
 */
export const THIRD_PARTY_HUB_DEMO_USE_CASE = Symbol('GetThirdPartyHubDemoUseCase');

/**
 * GetThirdPartyHubDemoUseCase — loads the third-party topic's hub
 * panel. Parameterless and repeatable: the sandbox runs are pure
 * functions of fixed scripts at a fixed exhibit clock, so each
 * execute serves byte-identical vendor panels while the wiring panel
 * follows the shared scenario store — which is itself part of the
 * lesson.
 */
export class GetThirdPartyHubDemoUseCase {
  private readonly repository: ThirdPartyHubRepository;

  constructor(repository: ThirdPartyHubRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ThirdPartyHubDemoData>> {
    return this.repository.get();
  }
}
