import type { Result } from '../../core/result';
import type { ArchTracerRepository } from './arch-tracer-repository';
import type { ArchTracerDemoData } from './arch-tracer';

/**
 * ARCH_TRACER_DEMO_USE_CASE — the container token this demo's use case
 * is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from
 * day one; the composition root registers the real pipeline under this
 * exact symbol:
 *
 *   container.register(ARCH_TRACER_DEMO_USE_CASE, new GetArchTracerDemoUseCase(repository));
 */
export const ARCH_TRACER_DEMO_USE_CASE = Symbol('GetArchTracerDemoUseCase');

/**
 * GetArchTracerDemoUseCase — loads the architecture topic's tracer
 * payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the server re-runs the traced request on every call,
 * so each load is a fresh walk under the scenario's current mode (flip
 * the mode, reload, and the same shape returns with a different leaf).
 */
export class GetArchTracerDemoUseCase {
  private readonly repository: ArchTracerRepository;

  constructor(repository: ArchTracerRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ArchTracerDemoData>> {
    return this.repository.get();
  }
}
