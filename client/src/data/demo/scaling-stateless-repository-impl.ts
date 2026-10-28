import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ScalingStatelessRepository } from '../../domain/demo/scaling-stateless-repository';
import type { ScalingStatelessDemoData } from '../../domain/demo/scaling-stateless';
import { toScalingStatelessDemo } from './scaling-stateless-dto';
import type { ScalingStatelessApi } from './scaling-stateless-api';

/**
 * ScalingStatelessRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so
 * the demo component never needs try/catch and no rejection goes
 * unhandled.
 */
export class ScalingStatelessRepositoryImpl implements ScalingStatelessRepository {
  private readonly api: ScalingStatelessApi;

  constructor(api: ScalingStatelessApi) {
    this.api = api;
  }

  async get(): Promise<Result<ScalingStatelessDemoData>> {
    return safeRepositoryCall(async () => toScalingStatelessDemo(await this.api.get()));
  }
}
