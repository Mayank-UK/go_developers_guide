import type { Result } from '../../core/result';
import type { PerfBenchRepository } from './perf-bench-repository';
import type { PerfBenchDemoData } from './perf-bench';

/**
 * PERF_BENCH_DEMO_USE_CASE — the container token this demo's use case
 * is registered under (same aliasing story as every demo's token); the
 * composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(PERF_BENCH_DEMO_USE_CASE, new GetPerfBenchDemoUseCase(repository));
 */
export const PERF_BENCH_DEMO_USE_CASE = Symbol('GetPerfBenchDemoUseCase');

/**
 * GetPerfBenchDemoUseCase — loads the performance topic's bench
 * payload. Parameterless and repeatable: the samples were parsed once
 * at boot, so the demo's reload gesture is "re-serve the cached run" —
 * the age moves with wall-clock time while no measured number does,
 * which is exactly the provenance story the panel tells.
 */
export class GetPerfBenchDemoUseCase {
  private readonly repository: PerfBenchRepository;

  constructor(repository: PerfBenchRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<PerfBenchDemoData>> {
    return this.repository.get();
  }
}
