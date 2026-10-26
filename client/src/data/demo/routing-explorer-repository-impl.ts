import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { RoutingExplorerRepository } from '../../domain/demo/routing-explorer-repository';
import type { RoutingExplorerDemoData } from '../../domain/demo/routing-explorer';
import { toRoutingExplorerDemo } from './routing-explorer-dto';
import type { RoutingExplorerApi } from './routing-explorer-api';

/**
 * RoutingExplorerRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class RoutingExplorerRepositoryImpl implements RoutingExplorerRepository {
  private readonly api: RoutingExplorerApi;

  constructor(api: RoutingExplorerApi) {
    this.api = api;
  }

  async get(): Promise<Result<RoutingExplorerDemoData>> {
    return safeRepositoryCall(async () => toRoutingExplorerDemo(await this.api.get()));
  }
}
