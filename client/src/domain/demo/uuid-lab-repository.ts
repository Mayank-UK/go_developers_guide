import type { Result } from '../../core/result';
import type { UUIDLabDemoData } from './uuid-lab';

/**
 * UUIDLabRepository — the domain-facing port for the sql-schema topic's
 * UUIDv7 lab payload. Demo components depend on this interface only; the
 * HTTP adapter lives in data/demo and is wired at the composition root.
 */
export interface UUIDLabRepository {
  /** A freshly minted batch with its measured properties. */
  get(): Promise<Result<UUIDLabDemoData>>;
}
