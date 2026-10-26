import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { MiddlewarePipelineRepository } from '../../domain/demo/middleware-pipeline-repository';
import type { MiddlewarePipelineDemoData } from '../../domain/demo/middleware-pipeline';
import { toMiddlewarePipelineDemo } from './middleware-pipeline-dto';
import type { MiddlewarePipelineApi } from './middleware-pipeline-api';

/**
 * MiddlewarePipelineRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so the
 * demo component never needs try/catch and no rejection goes unhandled.
 */
export class MiddlewarePipelineRepositoryImpl implements MiddlewarePipelineRepository {
  private readonly api: MiddlewarePipelineApi;

  constructor(api: MiddlewarePipelineApi) {
    this.api = api;
  }

  async get(): Promise<Result<MiddlewarePipelineDemoData>> {
    return safeRepositoryCall(async () => toMiddlewarePipelineDemo(await this.api.get()));
  }
}
