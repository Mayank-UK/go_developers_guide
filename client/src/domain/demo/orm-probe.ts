/**
 * Domain models for the sqlc-gorm topic's orm-probe demo — the
 * client-side shape of `GET /api/demo/db/orm-probe`. Field names match
 * the server's camelCase wire contract 1:1 (the DTO is the seam where
 * wire and domain may diverge later).
 *
 * The payload is a measurement: one seeded dataset, four trials of the
 * same logical job under different data-access styles, every SQL
 * statement counted. Statement counts, rows, and ratios are durable
 * facts; `durationMicros` and `timingBand` are the live observation of
 * the run that served this response — ranges, never promises.
 */

/** The seeded world the probe measured against. */
export interface ORMProbeDataset {
  /** Seeded posts — the N in N+1. */
  posts: number;
  /** Size of the seeded tag vocabulary. */
  tags: number;
  /** post↔tag edges (posts × tags per post). */
  edges: number;
}

/** One measured data-access strategy. */
export interface ORMProbeTrial {
  /** Fixed identity: pgx-baseline | sqlc-join | gorm-lazy | gorm-eager. */
  strategy: string;
  /** Human name of the trial. */
  label: string;
  /** The library behind the trial (pgx v5, sqlc, GORM). */
  library: string;
  /** The observability seam the statement count came from. */
  countingVia: string;
  /** Measured SQL statement count — the durable fact. */
  statements: number;
  /** Statements relative to the baseline, e.g. "1x", "13x". */
  roundTripRatio: string;
  /** Posts the trial answered. */
  rows: number;
  /** This run's wall time in microseconds (live, not a promise). */
  durationMicros: number;
  /** Coarse honest range: sub-millisecond | low-milliseconds | milliseconds. */
  timingBand: string;
  /** Up to three of the actual statements issued — the receipts. */
  sampleStatements: string[];
}

/** One observable step of the unit-of-work exhibit. */
export interface ORMProbeStep {
  action: string;
  detail: string;
}

/** The transactions-across-repositories exhibit: two repos, one tx,
 * a deliberate failure, nothing leaked. */
export interface ORMProbeUnitOfWork {
  repositories: string[];
  steps: ORMProbeStep[];
  hookApplied: boolean;
  postsBefore: number;
  postsAfter: number;
  rolledBack: boolean;
}

export interface ORMProbeDemoData {
  dataset: ORMProbeDataset;
  trials: ORMProbeTrial[];
  resultsAgree: boolean;
  unitOfWork: ORMProbeUnitOfWork;
}
