import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ThirdPartyHubRepository } from '../../domain/demo/thirdparty-hub-repository';
import type { ThirdPartyHubDemoData } from '../../domain/demo/thirdparty-hub';
import { toThirdPartyHubDemo } from './thirdparty-hub-dto';
import type { ThirdPartyHubApi } from './thirdparty-hub-api';

/**
 * ThirdPartyHubRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so
 * the demo component never needs try/catch and no rejection goes
 * unhandled.
 */
export class ThirdPartyHubRepositoryImpl implements ThirdPartyHubRepository {
  private readonly api: ThirdPartyHubApi;

  constructor(api: ThirdPartyHubApi) {
    this.api = api;
  }

  async get(): Promise<Result<ThirdPartyHubDemoData>> {
    return safeRepositoryCall(async () => toThirdPartyHubDemo(await this.api.get()));
  }
}
