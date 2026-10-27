import type { Result } from '../../core/result';
import type { MessagingOutboxRepository } from './messaging-outbox-repository';
import type { MessagingOutboxDemoData } from './messaging-outbox';

/**
 * MESSAGING_OUTBOX_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(MESSAGING_OUTBOX_DEMO_USE_CASE, new GetMessagingOutboxDemoUseCase(repository));
 */
export const MESSAGING_OUTBOX_DEMO_USE_CASE = Symbol('GetMessagingOutboxDemoUseCase');

/**
 * GetMessagingOutboxDemoUseCase — loads the messaging topic's outbox
 * story payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the server mints the story's throwaway namespace per
 * request.
 */
export class GetMessagingOutboxDemoUseCase {
  private readonly repository: MessagingOutboxRepository;

  constructor(repository: MessagingOutboxRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<MessagingOutboxDemoData>> {
    return this.repository.get();
  }
}
