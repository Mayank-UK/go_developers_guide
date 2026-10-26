import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { RaceRepository } from '../../domain/demo/db-race-repository';
import type { RaceDemoData } from '../../domain/demo/db-race';
import { toRaceDemo } from './db-race-dto';
import type { RaceApi } from './db-race-api';

/** RaceRepositoryImpl — adapts the demo API to the domain port. */
export class RaceRepositoryImpl implements RaceRepository {
  private readonly api: RaceApi;

  constructor(api: RaceApi) {
    this.api = api;
  }

  async get(): Promise<Result<RaceDemoData>> {
    return safeRepositoryCall(async () => toRaceDemo(await this.api.get()));
  }
}
