import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ObsTraceRepository } from '../../domain/demo/obs-trace-repository';
import type { ObsTraceDemoData } from '../../domain/demo/obs-trace';
import { toObsTraceDemo } from './obs-trace-dto';
import type { ObsTraceApi } from './obs-trace-api';

/**
 * ObsTraceRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload)
 * is guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class ObsTraceRepositoryImpl implements ObsTraceRepository {
  private readonly api: ObsTraceApi;

  constructor(api: ObsTraceApi) {
    this.api = api;
  }

  async get(): Promise<Result<ObsTraceDemoData>> {
    return safeRepositoryCall(async () => toObsTraceDemo(await this.api.get()));
  }
}
