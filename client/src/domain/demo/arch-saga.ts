/**
 * Domain models for the advanced-architecture topic's saga demo (demo
 * block id `arch:saga`) — the client-side shape of
 * `GET /api/demo/arch/saga`. Field names match the server's camelCase
 * wire contract 1:1.
 *
 * The payload is REAL machinery observed on the very request that
 * served it: the server drives three fresh runs of the real
 * patterns.SagaRunner (from server/internal/domain/patterns/saga.go)
 * through the fixed order-fulfillment script — a success run, a
 * failure run with deterministic injection at the shipment step plus
 * the idempotent re-compensation replay, and a mid-saga cancellation
 * run whose undo pass survives the dead context. Every ledger row,
 * world line, trigger, and count was recorded by the run that served
 * the payload; no clock or randomness participates, so replays serve
 * identical bodies.
 */

/** One step of the fixed script: its 0-based order, its forward effect, its undo effect. */
export interface ArchSagaStepRow {
  order: number;
  name: string;
  forward: string;
  undo: string;
}

/** One ledger row: the recording sequence, the step, the phase, the outcome, the detail. */
export interface ArchSagaLedgerRow {
  seq: number;
  step: string;
  phase: string;
  outcome: string;
  detail: string;
}

/** One counted observation — counted from the run's own ledger, never asserted. */
export interface ArchSagaCountRow {
  label: string;
  value: number;
}

/** One observed run: the script that drove it, the ledger, the world log, the counts, the replay. */
export interface ArchSagaRunPanel {
  id: string;
  title: string;
  lead: string;
  script: string;
  status: string;
  trigger?: string;
  ledger: ArchSagaLedgerRow[];
  worldLog: string[];
  counts: ArchSagaCountRow[];
  replay?: ArchSagaLedgerRow[];
  note: string;
}

/** One exported id of the real machinery, served as data. */
export interface ArchSagaSurfaceRow {
  id: string;
  kind: string;
  role: string;
}

/** The whole saga demo payload. */
export interface ArchSagaDemoData {
  mode: string;
  source: string;
  honestyNote: string;
  steps: ArchSagaStepRow[];
  runs: ArchSagaRunPanel[];
  semantics: string[];
  surface: ArchSagaSurfaceRow[];
}
