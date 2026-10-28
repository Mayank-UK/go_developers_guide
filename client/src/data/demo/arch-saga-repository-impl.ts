import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ArchSagaRepository } from '../../domain/demo/arch-saga-repository';
import type { ArchSagaDemoData } from '../../domain/demo/arch-saga';
import { toArchSagaDemo } from './arch-saga-dto';
import type { ArchSagaApi } from './arch-saga-api';

/**
 * ArchSagaRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class ArchSagaRepositoryImpl implements ArchSagaRepository {
  private readonly api: ArchSagaApi;

  constructor(api: ArchSagaApi) {
    this.api = api;
  }

  async get(): Promise<Result<ArchSagaDemoData>> {
    return safeRepositoryCall(async () => toArchSagaDemo(await this.api.get()));
  }
}
