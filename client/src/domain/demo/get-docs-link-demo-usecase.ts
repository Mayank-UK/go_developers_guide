import type { Result } from '../../core/result';
import type { DocsLinkRepository } from './docs-link-repository';
import type { DocsLinkDemoData } from './docs-link';

/**
 * API_DOCS_LINK_DEMO_USE_CASE — the container token this demo's use case
 * is registered under. Lives here (not in container.ts's ContainerTokens)
 * so the demo component has its resolution seam from day one; the
 * composition root registers the real pipeline under this exact symbol:
 *
 *   container.register(API_DOCS_LINK_DEMO_USE_CASE, new GetDocsLinkDemoUseCase(repository));
 */
export const API_DOCS_LINK_DEMO_USE_CASE = Symbol('GetDocsLinkDemoUseCase');

/**
 * GetDocsLinkDemoUseCase — loads the api-documentation topic's docs-link
 * demo payload: the OpenAPI contract as data, every fact parsed
 * server-side from the actual spec file. Thin orchestration seam over the
 * repository port (react-guide use-case style); parameterless because the
 * demo has no user inputs — the card is derived, not driven.
 */
export class GetDocsLinkDemoUseCase {
  private readonly repository: DocsLinkRepository;

  constructor(repository: DocsLinkRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<DocsLinkDemoData>> {
    return this.repository.get();
  }
}
