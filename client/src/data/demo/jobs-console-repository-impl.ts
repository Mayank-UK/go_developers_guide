import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { JobsConsoleRepository } from '../../domain/demo/jobs-console-repository';
import type { JobsConsoleDemoData } from '../../domain/demo/jobs-console';
import { toJobsConsoleDemo } from './jobs-console-dto';
import type { JobsConsoleApi } from './jobs-console-api';

/**
 * JobsConsoleRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class JobsConsoleRepositoryImpl implements JobsConsoleRepository {
  private readonly api: JobsConsoleApi;

  constructor(api: JobsConsoleApi) {
    this.api = api;
  }

  async get(): Promise<Result<JobsConsoleDemoData>> {
    return safeRepositoryCall(async () => toJobsConsoleDemo(await this.api.get()));
  }
}
