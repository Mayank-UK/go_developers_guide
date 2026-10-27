import type { Result } from '../../core/result';
import type { I18nStoryRepository } from './i18n-story-repository';
import type { I18nStoryDemoData } from './i18n-story';

/**
 * The container token for the i18n topic's negotiator demo, so the
 * component has its resolution seam from day one; the composition root
 * registers a GetI18nNegotiatorDemoUseCase under it.
 */
export const I18N_NEGOTIATOR_DEMO_USE_CASE = Symbol('GetI18nNegotiatorDemoUseCase');

/**
 * GetI18nNegotiatorDemoUseCase — loads the `i18n:negotiator` story
 * payload. Parameterless because the demo's input is ambient: the
 * request's own Accept-Language header rides the GET automatically
 * (the browser always sends one), and the server walks THAT header —
 * parsed candidates, the fallback chain, per-locale greetings, plural
 * rules, format matrices, the timezone beat, and the honest edges —
 * fresh per request.
 */
export class GetI18nNegotiatorDemoUseCase {
  private readonly repository: I18nStoryRepository;

  constructor(repository: I18nStoryRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<I18nStoryDemoData>> {
    return this.repository.get();
  }
}
