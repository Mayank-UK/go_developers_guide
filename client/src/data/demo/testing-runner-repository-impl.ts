import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { TestingRunnerRepository } from '../../domain/demo/testing-runner-repository';
import type { TestingRunnerDemoData } from '../../domain/demo/testing-runner';
import { toTestingRunnerDemo } from './testing-runner-dto';
import type { TestingRunnerApi } from './testing-runner-api';

/**
 * TestingRunnerRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload)
 * is guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class TestingRunnerRepositoryImpl implements TestingRunnerRepository {
  private readonly api: TestingRunnerApi;

  constructor(api: TestingRunnerApi) {
    this.api = api;
  }

  async get(): Promise<Result<TestingRunnerDemoData>> {
    return safeRepositoryCall(async () => toTestingRunnerDemo(await this.api.get()));
  }
}
