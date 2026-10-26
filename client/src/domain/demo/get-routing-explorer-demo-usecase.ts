import type { Result } from '../../core/result';
import type { RoutingExplorerRepository } from './routing-explorer-repository';
import type { RoutingExplorerDemoData } from './routing-explorer';

/**
 * ROUTING_EXPLORER_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(ROUTING_EXPLORER_DEMO_USE_CASE, new GetRoutingExplorerDemoUseCase(repository));
 */
export const ROUTING_EXPLORER_DEMO_USE_CASE = Symbol('GetRoutingExplorerDemoUseCase');

/**
 * GetRoutingExplorerDemoUseCase — loads the routing topic's explorer demo
 * payload. Thin orchestration seam over the repository port (react-guide
 * use-case style); parameterless because the demo has no user inputs —
 * the builder resolves a builder-chosen probe set against the route table
 * the server carries.
 */
export class GetRoutingExplorerDemoUseCase {
  private readonly repository: RoutingExplorerRepository;

  constructor(repository: RoutingExplorerRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<RoutingExplorerDemoData>> {
    return this.repository.get();
  }
}
