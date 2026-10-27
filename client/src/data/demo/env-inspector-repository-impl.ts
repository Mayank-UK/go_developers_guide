import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { EnvInspectorRepository } from '../../domain/demo/env-inspector-repository';
import type { EnvInspectorDemoData } from '../../domain/demo/env-inspector';
import { toEnvInspectorDemo } from './env-inspector-dto';
import type { EnvInspectorApi } from './env-inspector-api';

/**
 * EnvInspectorRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload)
 * is guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class EnvInspectorRepositoryImpl implements EnvInspectorRepository {
  private readonly api: EnvInspectorApi;

  constructor(api: EnvInspectorApi) {
    this.api = api;
  }

  async get(): Promise<Result<EnvInspectorDemoData>> {
    return safeRepositoryCall(async () => toEnvInspectorDemo(await this.api.get()));
  }
}
