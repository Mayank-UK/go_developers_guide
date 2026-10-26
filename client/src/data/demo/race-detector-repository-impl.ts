import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { RaceDetectorRepository } from '../../domain/demo/race-detector-repository';
import type { RaceDetectorDemoData } from '../../domain/demo/race-detector';
import { toRaceDetectorDemo } from './race-detector-dto';
import type { RaceDetectorApi } from './race-detector-api';

/**
 * RaceDetectorRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class RaceDetectorRepositoryImpl implements RaceDetectorRepository {
  private readonly api: RaceDetectorApi;

  constructor(api: RaceDetectorApi) {
    this.api = api;
  }

  async get(): Promise<Result<RaceDetectorDemoData>> {
    return safeRepositoryCall(async () => toRaceDetectorDemo(await this.api.get()));
  }
}
