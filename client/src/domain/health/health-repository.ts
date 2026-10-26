import type { Result } from '../../core/result';
import type { HealthStatus } from './health-status';

/**
 * HealthRepository — the domain-facing port for the liveness probe. Domain
 * code depends on this interface only; the adapter that talks HTTP lives in
 * data/health and is wired at the composition root.
 */
export interface HealthRepository {
  getHealth(): Promise<Result<HealthStatus>>;
}
