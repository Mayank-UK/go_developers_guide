import type { Result } from '../../core/result';
import type { SecurityStoryDemoData } from './security-story';

/**
 * SecurityStoryRepository — the domain-facing port for the security
 * topic's two story demos. Every demo component depends on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root (one adapter instance per demo endpoint path).
 */
export interface SecurityStoryRepository {
  /** The story's beats, proofs, and reasoning. */
  get(): Promise<Result<SecurityStoryDemoData>>;
}
