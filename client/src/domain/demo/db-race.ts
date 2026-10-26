/**
 * Domain models for the databases-sql topic's isolation demo — the
 * client-side shape of `GET /api/demo/db/race`. Field names match the
 * server's camelCase wire contract 1:1.
 *
 * The observed sequence is pinned by the choreography (channels and
 * wait-state polling, never sleeps), so every string is deterministic.
 */

/** One choreographed step of the two-transaction dance. */
export interface RaceStep {
  phase: string;
  actor: string;
  action: string;
  observed: string;
  note: string;
}

export interface RaceDemoData {
  isolation: string;
  scratchBefore: string;
  scratchAfter: string;
  steps: RaceStep[];
  lockWaited: boolean;
  serializableNote: string;
}
