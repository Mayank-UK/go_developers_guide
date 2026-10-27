import type { Result } from '../../core/result';
import type { I18nStoryDemoData } from './i18n-story';

/**
 * I18nStoryRepository — the domain-facing port for the i18n topic's
 * negotiator story demo. The demo component depends on this interface
 * only; the HTTP adapter lives in data/demo and is wired at the
 * composition root. The GET carries no inputs by design: the request's
 * own Accept-Language header (sent by the browser automatically) IS
 * the input, and the server derives every beat from it.
 */
export interface I18nStoryRepository {
  /** The story's beats, proofs, and reasoning. */
  get(): Promise<Result<I18nStoryDemoData>>;
}
