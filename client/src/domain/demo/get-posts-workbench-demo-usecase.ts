import type { Result } from '../../core/result';
import type { PostsWorkbenchRepository } from './posts-workbench-repository';
import type { PostsWorkbenchDemoData } from './posts-workbench';

/**
 * POSTS_WORKBENCH_DEMO_USE_CASE — the container token this demo's use case
 * is registered under. Lives here (not in container.ts's ContainerTokens)
 * so the demo component has its resolution seam from day one; the
 * composition root registers the real pipeline under this exact symbol.
 */
export const POSTS_WORKBENCH_DEMO_USE_CASE = Symbol('GetPostsWorkbenchDemoUseCase');

/**
 * GetPostsWorkbenchDemoUseCase — loads the api-design topic's contract
 * card. Thin orchestration seam over the repository port (react-guide
 * use-case style); parameterless because the payload is the server's
 * deterministic description of its own surface.
 */
export class GetPostsWorkbenchDemoUseCase {
  private readonly repository: PostsWorkbenchRepository;

  constructor(repository: PostsWorkbenchRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<PostsWorkbenchDemoData>> {
    return this.repository.get();
  }
}
