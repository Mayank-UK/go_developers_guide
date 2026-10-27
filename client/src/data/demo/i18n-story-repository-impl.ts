import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { I18nStoryRepository } from '../../domain/demo/i18n-story-repository';
import type { I18nStoryDemoData } from '../../domain/demo/i18n-story';
import { toI18nStoryDemo } from './i18n-story-dto';
import type { I18nStoryApi } from './i18n-story-api';

/**
 * I18nStoryRepositoryImpl — adapts the negotiator story API to the
 * domain port. Every thrown transport error (network, non-2xx,
 * malformed payload) is guarded by safeRepositoryCall into a Failed
 * Result, so the demo component never needs try/catch and no rejection
 * goes unhandled.
 */
export class I18nStoryRepositoryImpl implements I18nStoryRepository {
  private readonly api: I18nStoryApi;

  constructor(api: I18nStoryApi) {
    this.api = api;
  }

  async get(): Promise<Result<I18nStoryDemoData>> {
    return safeRepositoryCall(async () => toI18nStoryDemo(await this.api.get()));
  }
}
