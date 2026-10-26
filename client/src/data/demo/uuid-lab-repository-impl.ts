import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { UUIDLabRepository } from '../../domain/demo/uuid-lab-repository';
import type { UUIDLabDemoData } from '../../domain/demo/uuid-lab';
import { toUUIDLabDemo } from './uuid-lab-dto';
import type { UUIDLabApi } from './uuid-lab-api';

/**
 * UUIDLabRepositoryImpl — adapts the demo API to the domain port. Every
 * thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class UUIDLabRepositoryImpl implements UUIDLabRepository {
  private readonly api: UUIDLabApi;

  constructor(api: UUIDLabApi) {
    this.api = api;
  }

  async get(): Promise<Result<UUIDLabDemoData>> {
    return safeRepositoryCall(async () => toUUIDLabDemo(await this.api.get()));
  }
}
