import type { Result } from '../../core/result';
import type { MessagingStreamRepository } from './messaging-stream-repository';
import type { MessagingStreamDemoData } from './messaging-stream';

/**
 * MESSAGING_STREAM_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(MESSAGING_STREAM_DEMO_USE_CASE, new GetMessagingStreamDemoUseCase(repository));
 */
export const MESSAGING_STREAM_DEMO_USE_CASE = Symbol('GetMessagingStreamDemoUseCase');

/**
 * GetMessagingStreamDemoUseCase — loads the messaging topic's stream
 * story payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the server mints the story's throwaway namespace per
 * request.
 */
export class GetMessagingStreamDemoUseCase {
  private readonly repository: MessagingStreamRepository;

  constructor(repository: MessagingStreamRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<MessagingStreamDemoData>> {
    return this.repository.get();
  }
}
