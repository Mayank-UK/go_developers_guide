import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ReqresMethodsRepository } from '../../domain/demo/reqres-methods-repository';
import type { ReqresMethodsDemoData } from '../../domain/demo/reqres-methods';
import { toReqresMethodsDemo } from './reqres-methods-dto';
import type { ReqresMethodsApi } from './reqres-methods-api';

/**
 * ReqresMethodsRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class ReqresMethodsRepositoryImpl implements ReqresMethodsRepository {
  private readonly api: ReqresMethodsApi;

  constructor(api: ReqresMethodsApi) {
    this.api = api;
  }

  async get(): Promise<Result<ReqresMethodsDemoData>> {
    return safeRepositoryCall(async () => toReqresMethodsDemo(await this.api.get()));
  }
}
