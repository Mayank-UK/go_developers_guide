import type { Result } from '../../core/result';
import type { RoutingExplorerDemoData } from './routing-explorer';

/**
 * RoutingExplorerRepository — the domain-facing port for the routing
 * explorer demo payload. Demo components depend on this interface only;
 * the HTTP adapter lives in data/demo and is wired at the composition
 * root.
 */
export interface RoutingExplorerRepository {
  /** The real route table as data, the probe verdicts, exhibits, and semantics. */
  get(): Promise<Result<RoutingExplorerDemoData>>;
}
