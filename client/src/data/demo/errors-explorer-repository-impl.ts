import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ErrorsExplorerRepository } from '../../domain/demo/errors-explorer-repository';
import type { ErrorsExplorerDemoData } from '../../domain/demo/errors-explorer';
import { toErrorsExplorerDemo } from './errors-explorer-dto';
import type { ErrorsExplorerApi } from './errors-explorer-api';

/**
 * ErrorsExplorerRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so
 * the demo component never needs try/catch and no rejection goes
 * unhandled.
 */
export class ErrorsExplorerRepositoryImpl implements ErrorsExplorerRepository {
  private readonly api: ErrorsExplorerApi;

  constructor(api: ErrorsExplorerApi) {
    this.api = api;
  }

  async get(): Promise<Result<ErrorsExplorerDemoData>> {
    return safeRepositoryCall(async () => toErrorsExplorerDemo(await this.api.get()));
  }
}
