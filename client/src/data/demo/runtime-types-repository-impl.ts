import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { RuntimeTypesRepository } from '../../domain/demo/runtime-types-repository';
import type { RuntimeTypesDemoData } from '../../domain/demo/runtime-types';
import { toRuntimeTypesDemo } from './runtime-types-dto';
import type { RuntimeTypesApi } from './runtime-types-api';

/**
 * RuntimeTypesRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo component
 * never needs try/catch and no rejection goes unhandled.
 */
export class RuntimeTypesRepositoryImpl implements RuntimeTypesRepository {
  private readonly api: RuntimeTypesApi;

  constructor(api: RuntimeTypesApi) {
    this.api = api;
  }

  async get(): Promise<Result<RuntimeTypesDemoData>> {
    return safeRepositoryCall(async () => toRuntimeTypesDemo(await this.api.get()));
  }
}
