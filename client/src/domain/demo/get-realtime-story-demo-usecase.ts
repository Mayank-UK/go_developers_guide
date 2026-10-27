import type { Result } from '../../core/result';
import type { RealtimeStoryRepository } from './realtime-story-repository';
import type { RealtimeStoryDemoData } from './realtime-story';

/**
 * The two container tokens for the realtime topic's story demos — one
 * per demo, so each component resolves its own pipeline the family
 * way. They live here (not in container.ts's ContainerTokens) so the
 * demo components have their resolution seam from day one; the
 * composition root registers a GetRealtimeStoryDemoUseCase under each.
 */
export const REALTIME_TICKER_DEMO_USE_CASE = Symbol('GetRealtimeTickerDemoUseCase');
export const REALTIME_CHAT_DEMO_USE_CASE = Symbol('GetRealtimeChatDemoUseCase');

/**
 * GetRealtimeStoryDemoUseCase — loads one realtime story payload. The
 * two demos share one class (their payloads share one shape and the use
 * case has no per-demo logic); each instance is bound to ITS endpoint
 * through the repository the container wires under its own token.
 * Parameterless because the stories have no user inputs: the ticker
 * story derives its frames and resume math through the server's real
 * feed, the chat story runs a real hub choreography — every fetch tells
 * the whole story again.
 */
export class GetRealtimeStoryDemoUseCase {
  private readonly repository: RealtimeStoryRepository;

  constructor(repository: RealtimeStoryRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<RealtimeStoryDemoData>> {
    return this.repository.get();
  }
}
