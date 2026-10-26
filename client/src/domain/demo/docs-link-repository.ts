import type { Result } from '../../core/result';
import type { DocsLinkDemoData } from './docs-link';

/**
 * DocsLinkRepository — the domain-facing port for the docs-link demo
 * payload. Demo components depend on this interface only; the HTTP
 * adapter lives in data/demo and is wired at the composition root.
 */
export interface DocsLinkRepository {
  /** The spec facts card, parsed server-side from the real file. */
  get(): Promise<Result<DocsLinkDemoData>>;
}
