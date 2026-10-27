import type { Result } from '../../core/result';
import type { DepsManifestRepository } from './deps-manifest-repository';
import type { DepsManifestDemoData } from './deps-manifest';

/**
 * DEPS_MANIFEST_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(DEPS_MANIFEST_DEMO_USE_CASE, new GetDepsManifestDemoUseCase(repository));
 */
export const DEPS_MANIFEST_DEMO_USE_CASE = Symbol('GetDepsManifestDemoUseCase');

/**
 * GetDepsManifestDemoUseCase — loads the dependency-management topic's
 * manifest payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the server derives every fact at request time, and
 * the govulncheck panel may answer "running" (the scan is
 * backgrounded) on the first load.
 */
export class GetDepsManifestDemoUseCase {
  private readonly repository: DepsManifestRepository;

  constructor(repository: DepsManifestRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<DepsManifestDemoData>> {
    return this.repository.get();
  }
}
