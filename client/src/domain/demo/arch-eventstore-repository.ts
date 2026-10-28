import type { Result } from '../../core/result';
import type { ArchEventstoreDemoData } from './arch-eventstore';

/**
 * ArchEventstoreRepository — the domain-facing port for the
 * advanced-architecture topic's event-store exhibit. Demo components
 * depend on this interface only; the HTTP adapter lives in data/demo
 * and is wired at the composition root.
 */
export interface ArchEventstoreRepository {
  /** The event-store exhibit: three acts, the replay evidence, the store surface. */
  get(): Promise<Result<ArchEventstoreDemoData>>;
}
