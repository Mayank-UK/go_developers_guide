import type { Result } from '../../core/result';
import type { HttpHeadersRepository } from './http-headers-repository';
import type { HttpHeadersDemoData } from './http-headers';

/**
 * HTTP_HEADERS_DEMO_USE_CASE — the container token this demo's use case is
 * registered under. Lives here (not in container.ts's ContainerTokens) so
 * the demo component has its resolution seam from day one; the composition
 * root registers the real pipeline under this exact symbol:
 *
 *   container.register(HTTP_HEADERS_DEMO_USE_CASE, new GetHttpHeadersDemoUseCase(repository));
 */
export const HTTP_HEADERS_DEMO_USE_CASE = Symbol('GetHttpHeadersDemoUseCase');

/**
 * GetHttpHeadersDemoUseCase — loads the http-web topic's headers echo
 * demo payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the payload describes the fetch that fetched it.
 */
export class GetHttpHeadersDemoUseCase {
  private readonly repository: HttpHeadersRepository;

  constructor(repository: HttpHeadersRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<HttpHeadersDemoData>> {
    return this.repository.get();
  }
}
