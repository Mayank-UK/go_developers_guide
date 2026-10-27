import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { AuthzMatrixRepository } from '../../domain/demo/authz-matrix-repository';
import type { AuthzMatrixDemoData } from '../../domain/demo/authz-matrix';
import { toAuthzMatrixDemo } from './authz-matrix-dto';
import type { AuthzMatrixApi } from './authz-matrix-api';

/**
 * AuthzMatrixRepositoryImpl — adapts the matrix API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class AuthzMatrixRepositoryImpl implements AuthzMatrixRepository {
  private readonly api: AuthzMatrixApi;

  constructor(api: AuthzMatrixApi) {
    this.api = api;
  }

  async get(): Promise<Result<AuthzMatrixDemoData>> {
    return safeRepositoryCall(async () => toAuthzMatrixDemo(await this.api.get()));
  }
}
