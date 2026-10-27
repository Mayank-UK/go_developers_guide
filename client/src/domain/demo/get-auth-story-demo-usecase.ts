import type { Result } from '../../core/result';
import type { AuthStoryRepository } from './auth-story-repository';
import type { AuthStoryDemoData } from './auth-story';

/**
 * The seven container tokens for the authentication topic's story
 * demos — one per demo, so each component resolves its own pipeline the
 * family way. They live here (not in container.ts's ContainerTokens)
 * so the demo components have their resolution seam from day one; the
 * composition root registers a GetAuthStoryDemoUseCase under each.
 */
export const AUTH_SESSIONS_DEMO_USE_CASE = Symbol('GetAuthSessionsDemoUseCase');
export const AUTH_API_KEYS_DEMO_USE_CASE = Symbol('GetAuthAPIKeysDemoUseCase');
export const AUTH_COMPARE_DEMO_USE_CASE = Symbol('GetAuthCompareDemoUseCase');
export const AUTH_JWT_DEMO_USE_CASE = Symbol('GetAuthJWTDemoUseCase');
export const AUTH_OAUTH_DEMO_USE_CASE = Symbol('GetAuthOAuthDemoUseCase');
export const AUTH_HASHING_DEMO_USE_CASE = Symbol('GetAuthHashingDemoUseCase');
export const AUTH_REFRESH_DEMO_USE_CASE = Symbol('GetAuthRefreshDemoUseCase');

/**
 * GetAuthStoryDemoUseCase — loads one auth story payload. The seven
 * demos share one class (their payloads share one shape and the use
 * case has no per-demo logic); each instance is bound to ITS endpoint
 * through the repository the container wires under its own token.
 * Parameterless because the demos have no user inputs — the server
 * runs each story fresh per request.
 */
export class GetAuthStoryDemoUseCase {
  private readonly repository: AuthStoryRepository;

  constructor(repository: AuthStoryRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<AuthStoryDemoData>> {
    return this.repository.get();
  }
}
