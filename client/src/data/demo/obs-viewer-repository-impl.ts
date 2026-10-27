import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ObsViewerRepository } from '../../domain/demo/obs-viewer-repository';
import type { ObsViewerDemoData } from '../../domain/demo/obs-viewer';
import { toObsViewerDemo } from './obs-viewer-dto';
import type { ObsViewerApi } from './obs-viewer-api';

/**
 * ObsViewerRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload)
 * is guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class ObsViewerRepositoryImpl implements ObsViewerRepository {
  private readonly api: ObsViewerApi;

  constructor(api: ObsViewerApi) {
    this.api = api;
  }

  async get(): Promise<Result<ObsViewerDemoData>> {
    return safeRepositoryCall(async () => toObsViewerDemo(await this.api.get()));
  }
}
