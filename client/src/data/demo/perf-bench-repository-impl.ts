import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { PerfBenchRepository } from '../../domain/demo/perf-bench-repository';
import type { PerfBenchDemoData } from '../../domain/demo/perf-bench';
import { toPerfBenchDemo } from './perf-bench-dto';
import type { PerfBenchApi } from './perf-bench-api';

/**
 * PerfBenchRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload)
 * is guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class PerfBenchRepositoryImpl implements PerfBenchRepository {
  private readonly api: PerfBenchApi;

  constructor(api: PerfBenchApi) {
    this.api = api;
  }

  async get(): Promise<Result<PerfBenchDemoData>> {
    return safeRepositoryCall(async () => toPerfBenchDemo(await this.api.get()));
  }
}
