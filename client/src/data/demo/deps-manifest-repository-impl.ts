import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { DepsManifestRepository } from '../../domain/demo/deps-manifest-repository';
import type { DepsManifestDemoData } from '../../domain/demo/deps-manifest';
import { toDepsManifestDemo } from './deps-manifest-dto';
import type { DepsManifestApi } from './deps-manifest-api';

/**
 * DepsManifestRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so
 * the demo component never needs try/catch and no rejection goes
 * unhandled.
 */
export class DepsManifestRepositoryImpl implements DepsManifestRepository {
  private readonly api: DepsManifestApi;

  constructor(api: DepsManifestApi) {
    this.api = api;
  }

  async get(): Promise<Result<DepsManifestDemoData>> {
    return safeRepositoryCall(async () => toDepsManifestDemo(await this.api.get()));
  }
}
