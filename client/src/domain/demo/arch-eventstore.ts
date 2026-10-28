/**
 * Domain models for the advanced-architecture topic's event-store
 * exhibit (demo block id `arch:eventstore`) — the client-side shape of
 * `GET /api/demo/arch/eventstore`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload is REAL machinery observed on the very request that
 * served it: a fresh in-process event store (the server's
 * internal/domain/patterns package) driven through a fixed three-act
 * script — the command path (rebuild by replay, decide, append with the
 * expected version), an optimistic-concurrency collision with the
 * typed conflict and both halves of its recovery, and the rebuild
 * proofs (a byte-identical double replay checked against an independent
 * naive ledger, plus the absent stream's zero-state fold). Every
 * version, count, state, and conflict number was observed from the real
 * store; event occurrence times are fixed data at a fixed exhibit
 * clock, never wall-clock readings.
 */

/** One observed step of the stream's story: the attempt, the answer, the version after. */
export interface ArchEventstoreRow {
  step: number;
  call: string;
  input: string;
  outcome: string;
  /** The stream's version AFTER the step — always observed; 0 is real data (nothing has happened yet). */
  version: number;
  /** Carried only by the conflict step (the server's omitempty). */
  expectedVersion?: number;
  /** Carried only by the conflict step (the server's omitempty). */
  actualVersion?: number;
  detail: string;
}

/** One counted observation — counted during the run, never asserted. */
export interface ArchEventstoreCount {
  label: string;
  value: number;
}

/** The inventory item's state AS REBUILT FROM EVENTS — never stored anywhere. */
export interface ArchEventstoreInventoryState {
  onHand: number;
  reserved: number;
  available: number;
  /** Open reservations per order ref (the fold's per-ref bookkeeping). */
  reservations: Record<string, number>;
}

/** One panel as served: framing, rules, observed rows, counted totals. */
export interface ArchEventstorePanel {
  id: string;
  title: string;
  lead: string;
  config: string[];
  rows: ArchEventstoreRow[];
  counts: ArchEventstoreCount[];
}

/** The rebuild's proof block: fold order, state, naive ledger, double rebuild, absent stream. */
export interface ArchEventstoreReplayEvidence {
  foldedEvents: number;
  appliedInOrder: string[];
  state: ArchEventstoreInventoryState;
  naiveExpected: ArchEventstoreInventoryState;
  rebuildsRun: number;
  byteIdentical: boolean;
  absentStreamId: string;
  absentState: ArchEventstoreInventoryState;
  absentVersion: number;
  note: string;
}

/** The whole event-store exhibit payload. */
export interface ArchEventstoreDemoData {
  mode: string;
  honestyNote: string;
  streamId: string;
  store: string[];
  panels: ArchEventstorePanel[];
  replay: ArchEventstoreReplayEvidence;
}
