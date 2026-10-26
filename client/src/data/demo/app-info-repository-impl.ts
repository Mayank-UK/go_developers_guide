import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { AppInfoRepository } from '../../domain/demo/app-info-repository';
import type { AppInfoDemoData } from '../../domain/demo/app-info';
import { toAppInfoDemo } from './app-info-dto';
import type { AppInfoApi } from './app-info-api';

/**
 * AppInfoRepositoryImpl — adapts the app-info demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo component
 * never needs try/catch and no rejection goes unhandled.
 */
export class AppInfoRepositoryImpl implements AppInfoRepository {
  private readonly api: AppInfoApi;

  constructor(api: AppInfoApi) {
    this.api = api;
  }

  async get(): Promise<Result<AppInfoDemoData>> {
    return safeRepositoryCall(async () => toAppInfoDemo(await this.api.get()));
  }
}
