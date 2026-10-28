import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { ArchEventstoreRepository } from '../../domain/demo/arch-eventstore-repository';
import type { ArchEventstoreDemoData } from '../../domain/demo/arch-eventstore';
import { toArchEventstoreDemo } from './arch-eventstore-dto';
import type { ArchEventstoreApi } from './arch-eventstore-api';

/**
 * ArchEventstoreRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so
 * the demo component never needs try/catch and no rejection goes
 * unhandled.
 */
export class ArchEventstoreRepositoryImpl implements ArchEventstoreRepository {
  private readonly api: ArchEventstoreApi;

  constructor(api: ArchEventstoreApi) {
    this.api = api;
  }

  async get(): Promise<Result<ArchEventstoreDemoData>> {
    return safeRepositoryCall(async () => toArchEventstoreDemo(await this.api.get()));
  }
}
