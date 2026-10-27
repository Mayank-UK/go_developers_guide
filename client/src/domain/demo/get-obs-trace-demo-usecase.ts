import type { Result } from '../../core/result';
import type { ObsTraceRepository } from './obs-trace-repository';
import type { ObsTraceDemoData } from './obs-trace';

/**
 * OBS_TRACE_DEMO_USE_CASE — the container token this demo's use case
 * is registered under (same aliasing story as every demo's token); the
 * composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(OBS_TRACE_DEMO_USE_CASE, new GetObsTraceDemoUseCase(repository));
 */
export const OBS_TRACE_DEMO_USE_CASE = Symbol('GetObsTraceDemoUseCase');

/**
 * GetObsTraceDemoUseCase — loads the observability topic's trace
 * round-trip payload. Parameterless: the server re-runs the traced
 * walk on every call (under a minted traceparent when the browser
 * sends none), so each load is a fresh tree.
 */
export class GetObsTraceDemoUseCase {
  private readonly repository: ObsTraceRepository;

  constructor(repository: ObsTraceRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<ObsTraceDemoData>> {
    return this.repository.get();
  }
}
