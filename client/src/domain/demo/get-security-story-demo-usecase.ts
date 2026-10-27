import type { Result } from '../../core/result';
import type { SecurityStoryRepository } from './security-story-repository';
import type { SecurityStoryDemoData } from './security-story';

/**
 * The two container tokens for the security topic's story demos — one
 * per demo, so each component resolves its own pipeline the family
 * way. They live here (not in container.ts's ContainerTokens) so the
 * demo components have their resolution seam from day one; the
 * composition root registers a GetSecurityStoryDemoUseCase under each.
 */
export const SECURITY_LAB_DEMO_USE_CASE = Symbol('GetSecurityLabDemoUseCase');
export const SECURITY_PII_DEMO_USE_CASE = Symbol('GetSecurityPIIDemoUseCase');

/**
 * GetSecurityStoryDemoUseCase — loads one security story payload. The
 * two demos share one class (their payloads share one shape and the use
 * case has no per-demo logic); each instance is bound to ITS endpoint
 * through the repository the container wires under its own token.
 * Parameterless because the demos have no user inputs — the server
 * runs each story fresh per request (the lab story mints a fresh rate
 * limiter, the PII story a fresh subject store, so every run tells the
 * whole story again).
 */
export class GetSecurityStoryDemoUseCase {
  private readonly repository: SecurityStoryRepository;

  constructor(repository: SecurityStoryRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<SecurityStoryDemoData>> {
    return this.repository.get();
  }
}
