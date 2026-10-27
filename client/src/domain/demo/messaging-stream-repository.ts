import type { Result } from '../../core/result';
import type { MessagingStreamDemoData } from './messaging-stream';

/**
 * MessagingStreamRepository — the domain-facing port for the messaging
 * topic's stream-story demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface MessagingStreamRepository {
  /** The story's beats and proofs. */
  get(): Promise<Result<MessagingStreamDemoData>>;
}
