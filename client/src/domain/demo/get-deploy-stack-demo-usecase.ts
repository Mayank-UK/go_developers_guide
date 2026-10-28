import type { Result } from '../../core/result';
import type { DeployStackRepository } from './deploy-stack-repository';
import type { DeployStackDemoData } from './deploy-stack';

/**
 * DEPLOY_STACK_DEMO_USE_CASE — the container token this demo's use
 * case is registered under (same aliasing story as every demo's
 * token); the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(DEPLOY_STACK_DEMO_USE_CASE, new GetDeployStackDemoUseCase(repository));
 */
export const DEPLOY_STACK_DEMO_USE_CASE = Symbol('GetDeployStackDemoUseCase');

/**
 * GetDeployStackDemoUseCase — loads the deployment topic's stack
 * viewer payload. Parameterless and repeatable: every execute asks
 * the server to re-read and re-parse the deploy/ artifacts, so the
 * demo's reload gesture is "parse the declarations again" — edit the
 * compose file or a manifest, reload, watch the panel follow.
 */
export class GetDeployStackDemoUseCase {
  private readonly repository: DeployStackRepository;

  constructor(repository: DeployStackRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<DeployStackDemoData>> {
    return this.repository.get();
  }
}
