import type { Result } from '../../core/result';
import type { ContextCancellationDemoData } from './context-cancellation';

/**
 * ContextCancellationRepository — the domain-facing port for the
 * context-cancellation demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface ContextCancellationRepository {
  /** The cancellation choreography transcript, performed live by the server. */
  get(): Promise<Result<ContextCancellationDemoData>>;
}
