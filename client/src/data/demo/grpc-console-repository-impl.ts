import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { GRPCConsoleRepository } from '../../domain/demo/grpc-console-repository';
import type { GRPCConsoleDemoData } from '../../domain/demo/grpc-console';
import { toGRPCConsoleDemo } from './grpc-console-dto';
import type { GRPCConsoleApi } from './grpc-console-api';

/**
 * GRPCConsoleRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class GRPCConsoleRepositoryImpl implements GRPCConsoleRepository {
  private readonly api: GRPCConsoleApi;

  constructor(api: GRPCConsoleApi) {
    this.api = api;
  }

  async get(): Promise<Result<GRPCConsoleDemoData>> {
    return safeRepositoryCall(async () => toGRPCConsoleDemo(await this.api.get()));
  }
}
