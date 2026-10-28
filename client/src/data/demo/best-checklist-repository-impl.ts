import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { BestChecklistRepository } from '../../domain/demo/best-checklist-repository';
import type { BestChecklistDemoData } from '../../domain/demo/best-checklist';
import { toBestChecklistDemo } from './best-checklist-dto';
import type { BestChecklistApi } from './best-checklist-api';

/**
 * BestChecklistRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so
 * the demo component never needs try/catch and no rejection goes
 * unhandled.
 */
export class BestChecklistRepositoryImpl implements BestChecklistRepository {
  private readonly api: BestChecklistApi;

  constructor(api: BestChecklistApi) {
    this.api = api;
  }

  async get(): Promise<Result<BestChecklistDemoData>> {
    return safeRepositoryCall(async () => toBestChecklistDemo(await this.api.get()));
  }
}
