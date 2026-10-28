import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { DeployStackRepository } from '../../domain/demo/deploy-stack-repository';
import type { DeployStackDemoData } from '../../domain/demo/deploy-stack';
import { toDeployStackDemo } from './deploy-stack-dto';
import type { DeployStackApi } from './deploy-stack-api';

/**
 * DeployStackRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload)
 * is guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class DeployStackRepositoryImpl implements DeployStackRepository {
  private readonly api: DeployStackApi;

  constructor(api: DeployStackApi) {
    this.api = api;
  }

  async get(): Promise<Result<DeployStackDemoData>> {
    return safeRepositoryCall(async () => toDeployStackDemo(await this.api.get()));
  }
}
