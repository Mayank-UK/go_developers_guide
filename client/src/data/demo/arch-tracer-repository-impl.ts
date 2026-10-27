import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ArchTracerRepository } from '../../domain/demo/arch-tracer-repository';
import type { ArchTracerDemoData } from '../../domain/demo/arch-tracer';
import { toArchTracerDemo } from './arch-tracer-dto';
import type { ArchTracerApi } from './arch-tracer-api';

/**
 * ArchTracerRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload)
 * is guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class ArchTracerRepositoryImpl implements ArchTracerRepository {
  private readonly api: ArchTracerApi;

  constructor(api: ArchTracerApi) {
    this.api = api;
  }

  async get(): Promise<Result<ArchTracerDemoData>> {
    return safeRepositoryCall(async () => toArchTracerDemo(await this.api.get()));
  }
}
