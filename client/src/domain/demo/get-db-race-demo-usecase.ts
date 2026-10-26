import type { Result } from '../../core/result';
import type { RaceRepository } from './db-race-repository';
import type { RaceDemoData } from './db-race';

/** DB_RACE_DEMO_USE_CASE — the container token for this demo's use case. */
export const DB_RACE_DEMO_USE_CASE = Symbol('GetRaceDemoUseCase');

/** GetRaceDemoUseCase — loads the isolation demo payload. */
export class GetRaceDemoUseCase {
  private readonly repository: RaceRepository;

  constructor(repository: RaceRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<RaceDemoData>> {
    return this.repository.get();
  }
}
