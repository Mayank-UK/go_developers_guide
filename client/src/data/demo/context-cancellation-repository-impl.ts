import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ContextCancellationRepository } from '../../domain/demo/context-cancellation-repository';
import type { ContextCancellationDemoData } from '../../domain/demo/context-cancellation';
import { toContextCancellationDemo } from './context-cancellation-dto';
import type { ContextCancellationApi } from './context-cancellation-api';

/**
 * ContextCancellationRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so the
 * demo component never needs try/catch and no rejection goes unhandled.
 */
export class ContextCancellationRepositoryImpl implements ContextCancellationRepository {
  private readonly api: ContextCancellationApi;

  constructor(api: ContextCancellationApi) {
    this.api = api;
  }

  async get(): Promise<Result<ContextCancellationDemoData>> {
    return safeRepositoryCall(async () => toContextCancellationDemo(await this.api.get()));
  }
}
