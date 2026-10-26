import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ValidationArenaRepository } from '../../domain/demo/validation-arena-repository';
import type { ValidationArenaDemoData } from '../../domain/demo/validation-arena';
import { toValidationArenaDemo } from './validation-arena-dto';
import type { ValidationArenaApi } from './validation-arena-api';

/**
 * ValidationArenaRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class ValidationArenaRepositoryImpl implements ValidationArenaRepository {
  private readonly api: ValidationArenaApi;

  constructor(api: ValidationArenaApi) {
    this.api = api;
  }

  async get(): Promise<Result<ValidationArenaDemoData>> {
    return safeRepositoryCall(async () => toValidationArenaDemo(await this.api.get()));
  }
}
