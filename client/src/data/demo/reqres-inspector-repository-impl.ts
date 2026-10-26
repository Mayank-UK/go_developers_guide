import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ReqresInspectorRepository } from '../../domain/demo/reqres-inspector-repository';
import type { ReqresInspectorDemoData } from '../../domain/demo/reqres-inspector';
import { toReqresInspectorDemo } from './reqres-inspector-dto';
import type { ReqresInspectorApi } from './reqres-inspector-api';

/**
 * ReqresInspectorRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class ReqresInspectorRepositoryImpl implements ReqresInspectorRepository {
  private readonly api: ReqresInspectorApi;

  constructor(api: ReqresInspectorApi) {
    this.api = api;
  }

  async get(): Promise<Result<ReqresInspectorDemoData>> {
    return safeRepositoryCall(async () => toReqresInspectorDemo(await this.api.get()));
  }
}
