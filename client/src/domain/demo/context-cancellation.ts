/**
 * Domain models for the goroutines-and-channels topic's
 * context-cancellation demo — the client-side shape of
 * `GET /api/demo/conc/context`. Field names match the server's camelCase
 * wire contract 1:1 (the DTO is the seam where wire and domain may
 * diverge later).
 */

/** One derivation edge: `to` was derived from `from`. */
export interface TreeEdge {
  from: string;
  to: string;
}

/** The choreography's configuration, as facts: budget, trigger, tree. */
export interface ChoreographyFacts {
  /** The deadline armed on the parent context, in milliseconds. */
  deadlineBudgetMs: number;
  /** Work items each worker completes before the cancel lands. */
  workItemsPerWorker: number;
  /** What closed Done(): the explicit cancel (or the deadline). */
  trigger: string;
  /** The context derivation tree the cancellation propagates through. */
  tree: TreeEdge[];
  /** How the recorded observation order was produced. */
  ordering: string;
}

/** One worker's completed work before cancellation landed. */
export interface WorkRow {
  workerId: string;
  generation: number;
  itemsProcessed: number;
}

/** One worker's observation of cancellation, with its recorded position. */
export interface PropagationRow {
  workerId: string;
  generation: number;
  /** The context this worker was derived from ("parent", "alpha", …). */
  derivedFrom: string;
  /** 1-based position in the choreography's recorded order. */
  observedPosition: number;
  /** What ctx.Err() reported: "context canceled" or the deadline cause. */
  cause: string;
}

/** The beat where nobody cancels: the deadline fires on its own. */
export interface DeadlineOutcome {
  budgetMs: number;
  cancelCalled: boolean;
  outcome: string;
  cause: string;
  /** errors.Is(ctx.Err(), context.DeadlineExceeded) — the production check. */
  deadlineExceeded: boolean;
}

/** The hygiene books: goroutines joined, contexts cancelled, no leaks. */
export interface CancellationLedger {
  goroutinesLaunched: number;
  goroutinesJoined: number;
  contextsCreated: number;
  contextsCancelled: number;
  outcome: string;
}

export interface ContextCancellationDemoData {
  choreography: ChoreographyFacts;
  workPhase: WorkRow[];
  propagation: PropagationRow[];
  deadlineBeat: DeadlineOutcome;
  ledger: CancellationLedger;
}
