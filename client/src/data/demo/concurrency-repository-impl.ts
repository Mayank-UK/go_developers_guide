import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ConcurrencyRepository } from '../../domain/demo/concurrency-repository';
import type { ConcurrencyDemoData } from '../../domain/demo/concurrency';
import { toConcurrencyDemo } from './concurrency-dto';
import type { ConcurrencyApi } from './concurrency-api';

/**
 * ConcurrencyRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class ConcurrencyRepositoryImpl implements ConcurrencyRepository {
  private readonly api: ConcurrencyApi;

  constructor(api: ConcurrencyApi) {
    this.api = api;
  }

  async get(): Promise<Result<ConcurrencyDemoData>> {
    return safeRepositoryCall(async () => toConcurrencyDemo(await this.api.get()));
  }
}
