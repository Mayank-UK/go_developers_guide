import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ORMProbeRepository } from '../../domain/demo/orm-probe-repository';
import type { ORMProbeDemoData } from '../../domain/demo/orm-probe';
import { toORMProbeDemo } from './orm-probe-dto';
import type { ORMProbeApi } from './orm-probe-api';

/**
 * ORMProbeRepositoryImpl — adapts the demo API to the domain port. Every
 * thrown transport error (network, non-2xx, malformed payload) is guarded
 * by safeRepositoryCall into a Failed Result, so the demo component never
 * needs try/catch and no rejection goes unhandled.
 */
export class ORMProbeRepositoryImpl implements ORMProbeRepository {
  private readonly api: ORMProbeApi;

  constructor(api: ORMProbeApi) {
    this.api = api;
  }

  async get(): Promise<Result<ORMProbeDemoData>> {
    return safeRepositoryCall(async () => toORMProbeDemo(await this.api.get()));
  }
}
