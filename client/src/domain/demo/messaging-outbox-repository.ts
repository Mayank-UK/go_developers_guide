import type { Result } from '../../core/result';
import type { MessagingOutboxDemoData } from './messaging-outbox';

/**
 * MessagingOutboxRepository — the domain-facing port for the messaging
 * topic's outbox-story demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface MessagingOutboxRepository {
  /** The story's beats and proofs. */
  get(): Promise<Result<MessagingOutboxDemoData>>;
}
