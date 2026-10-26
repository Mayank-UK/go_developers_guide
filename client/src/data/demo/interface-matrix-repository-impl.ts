import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { InterfaceMatrixRepository } from '../../domain/demo/interface-matrix-repository';
import type { InterfaceMatrixDemoData } from '../../domain/demo/interface-matrix';
import { toInterfaceMatrixDemo } from './interface-matrix-dto';
import type { InterfaceMatrixApi } from './interface-matrix-api';

/**
 * InterfaceMatrixRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class InterfaceMatrixRepositoryImpl implements InterfaceMatrixRepository {
  private readonly api: InterfaceMatrixApi;

  constructor(api: InterfaceMatrixApi) {
    this.api = api;
  }

  async get(): Promise<Result<InterfaceMatrixDemoData>> {
    return safeRepositoryCall(async () => toInterfaceMatrixDemo(await this.api.get()));
  }
}
