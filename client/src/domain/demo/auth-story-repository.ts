import type { Result } from '../../core/result';
import type { AuthStoryDemoData } from './auth-story';

/**
 * AuthStoryRepository — the domain-facing port for the authentication
 * topic's seven story demos. Every demo component depends on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root (one adapter instance per demo endpoint path).
 */
export interface AuthStoryRepository {
  /** The story's beats, proofs, and reasoning. */
  get(): Promise<Result<AuthStoryDemoData>>;
}
