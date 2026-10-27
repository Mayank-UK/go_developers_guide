import type { Result } from '../../core/result';
import type { DepsManifestDemoData } from './deps-manifest';

/**
 * DepsManifestRepository — the domain-facing port for the
 * dependency-management topic's manifest demo payload. Demo components
 * depend on this interface only; the HTTP adapter lives in data/demo
 * and is wired at the composition root.
 */
export interface DepsManifestRepository {
  /** The live manifest + build info + anchors + caches + vulncheck. */
  get(): Promise<Result<DepsManifestDemoData>>;
}
