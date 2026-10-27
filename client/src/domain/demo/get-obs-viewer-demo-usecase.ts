import type { Result } from '../../core/result';
import type { ObsViewerRepository } from './obs-viewer-repository';
import type { ObsViewerDemoData } from './obs-viewer';

/**
 * OBS_VIEWER_DEMO_USE_CASE — the container token this demo's use case
 * is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(OBS_VIEWER_DEMO_USE_CASE, new GetObsViewerDemoUseCase(repository));
 */
export const OBS_VIEWER_DEMO_USE_CASE = Symbol('GetObsViewerDemoUseCase');

/**
 * GetObsViewerDemoUseCase — loads the observability topic's slog
 * viewer payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the server re-runs the replay on every call, so each
 * load is a fresh capture with fresh (real) durations.
 */
export class GetObsViewerDemoUseCase {
  private readonly repository: ObsViewerRepository;

  constructor(repository: ObsViewerRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ObsViewerDemoData>> {
    return this.repository.get();
  }
}
