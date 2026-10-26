import type { Result } from '../../core/result';
import type { RaceDemoData } from './db-race';

/** RaceRepository — the domain-facing port for the isolation demo payload. */
export interface RaceRepository {
  /** The choreographed two-transaction story. */
  get(): Promise<Result<RaceDemoData>>;
}
