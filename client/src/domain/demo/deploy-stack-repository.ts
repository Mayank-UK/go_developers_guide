import type { Result } from '../../core/result';
import type { DeployStackDemoData } from './deploy-stack';

/**
 * DeployStackRepository — the domain-facing port for the deployment
 * topic's stack viewer payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface DeployStackRepository {
  /** The repo's deployment artifacts, parsed and derived by the server. */
  get(): Promise<Result<DeployStackDemoData>>;
}
