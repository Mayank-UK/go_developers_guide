/**
 * Domain models for the scaling topic's statelessness demo (demo block
 * id `scaling:stateless`) — the client-side shape of
 * `GET /api/demo/scaling/stateless`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload is a MEASURED round, not narration: the server builds
 * three in-process replicas behind a real round-robin balancer with an
 * affinity (sticky) table, plus a shared store standing for
 * externalized state, and walks one session through both placements.
 * DISTRIBUTION carries the measured per-replica served counts;
 * STEPS is the thirteen-step session round (local memory breaking on
 * hop and swap, sticky routing's lease, the shared placement surviving
 * everything, the counter beat); VERDICTS state what the round proved
 * with its measured numbers. The round is deterministic — no clock, no
 * randomness — so the payload replays identically on every request.
 */

/** The round's config panel: the constants it ran under. */
export interface ScalingConfigPanel {
  replicas: number;
  distributionRequests: number;
  clientId: string;
  counterIncrements: number;
  localToken: string;
  sharedToken: string;
  sharedStoreNote: string;
}

/** One replica's measured share of the distribution act. */
export interface ScalingDistributionRow {
  replicaId: string;
  served: number;
}

/** The distribution act: measured total, rows, and derived note. */
export interface ScalingDistributionPanel {
  totalServed: number;
  rows: ScalingDistributionRow[];
  note: string;
}

/** One step of the session round, with its measured observation. */
export interface ScalingStepView {
  step: number;
  phase: string;
  label: string;
  route: string;
  replicaId: string;
  outcome: string;
  detail: string;
  observed: string;
}

/** One replica's measured share of the counter act. */
export interface ScalingCounterRow {
  replicaId: string;
  increments: number;
}

/** The counter act: per-replica increments and the shared total. */
export interface ScalingCounterPanel {
  rows: ScalingCounterRow[];
  total: number;
  note: string;
}

/** One thing the round proved, stated with its measured numbers. */
export interface ScalingVerdictView {
  id: string;
  label: string;
  value: string;
}

/** The whole statelessness payload. */
export interface ScalingStatelessDemoData {
  mode: string;
  config: ScalingConfigPanel;
  distribution: ScalingDistributionPanel;
  steps: ScalingStepView[];
  counter: ScalingCounterPanel;
  verdicts: ScalingVerdictView[];
  honesty: string;
}
