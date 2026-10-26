import type { Result } from '../../core/result';
import type { AppInfoRepository } from './app-info-repository';
import type { AppInfoDemoData } from './app-info';

/**
 * The container registration token for this use case. The wave authoring
 * pattern lands the container's own token pair + getter at wave close
 * (container.ts is master-owned); this exported symbol is the stable seam
 * the UI hook resolves through in the meantime, and container wiring can
 * register the real pipeline under it in one line:
 *
 *     this.register(GetAppInfoDemoUseCaseToken, new GetAppInfoDemoUseCase(...))
 *
 * Specs replace the registration through the same token (the container's
 * documented replace-a-singleton seam).
 */
export const GetAppInfoDemoUseCaseToken = Symbol('GetAppInfoDemoUseCase');

/**
 * GetAppInfoDemoUseCase — loads the go-http-fundamentals topic's app-info
 * demo payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * inputs — the server introspects itself.
 */
export class GetAppInfoDemoUseCase {
  private readonly repository: AppInfoRepository;

  constructor(repository: AppInfoRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<AppInfoDemoData>> {
    return this.repository.get();
  }
}
