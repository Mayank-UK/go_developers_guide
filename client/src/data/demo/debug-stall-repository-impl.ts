import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { DebugStallRepository } from '../../domain/demo/debug-stall-repository';
import type { DebugStallDemoData } from '../../domain/demo/debug-stall';
import { toDebugStallDemo } from './debug-stall-dto';
import type { DebugStallApi } from './debug-stall-api';

/**
 * DebugStallRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload)
 * is guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class DebugStallRepositoryImpl implements DebugStallRepository {
  private readonly api: DebugStallApi;

  constructor(api: DebugStallApi) {
    this.api = api;
  }

  async get(): Promise<Result<DebugStallDemoData>> {
    return safeRepositoryCall(async () => toDebugStallDemo(await this.api.get()));
  }
}
