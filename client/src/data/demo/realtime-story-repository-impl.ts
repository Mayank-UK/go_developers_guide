import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { RealtimeStoryRepository } from '../../domain/demo/realtime-story-repository';
import type { RealtimeStoryDemoData } from '../../domain/demo/realtime-story';
import { toRealtimeStoryDemo } from './realtime-story-dto';
import type { RealtimeStoryApi } from './realtime-story-api';

/**
 * RealtimeStoryRepositoryImpl — adapts one story API instance to the
 * domain port. Every thrown transport error (network, non-2xx,
 * malformed payload) is guarded by safeRepositoryCall into a Failed
 * Result, so the demo components never need try/catch and no rejection
 * goes unhandled.
 */
export class RealtimeStoryRepositoryImpl implements RealtimeStoryRepository {
  private readonly api: RealtimeStoryApi;

  constructor(api: RealtimeStoryApi) {
    this.api = api;
  }

  async get(): Promise<Result<RealtimeStoryDemoData>> {
    return safeRepositoryCall(async () => toRealtimeStoryDemo(await this.api.get()));
  }
}
