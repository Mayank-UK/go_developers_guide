import type { Result } from '../../core/result';
import type { RealtimeStoryDemoData } from './realtime-story';

/**
 * RealtimeStoryRepository — the domain-facing port for the realtime
 * topic's two story demos. Every demo component depends on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root (one adapter instance per demo endpoint path).
 */
export interface RealtimeStoryRepository {
  /** The story's beats, proofs, and reasoning. */
  get(): Promise<Result<RealtimeStoryDemoData>>;
}
