/**
 * Domain models for the go-runtime topic's concurrency demo — the
 * client-side shape of `GET /api/demo/runtime/concurrency`. Field names
 * match the server's camelCase wire contract 1:1 (the DTO is the seam
 * where wire and domain may diverge later).
 */

/** One job's journey through the pool, sorted by job id. */
export interface JobRow {
  /** Zero-padded stable id, e.g. `job-01`. */
  jobId: string;
  /** The workload that went in, e.g. `alpha`. */
  payload: string;
  /** The deterministic transform output, e.g. `ALPHA#518`. */
  output: string;
}

/** Observed per-worker tally — the one fact that varies run to run. */
export interface WorkerRow {
  /** Stable id, e.g. `worker-2`. */
  workerId: string;
  /** How many of the jobs this worker won this run. */
  jobsDone: number;
}

/** The choreography's configuration — identical on every call. */
export interface PoolSetup {
  jobs: number;
  workers: number;
  cancelAfter: number;
  strategy: string;
}

/** The context-cancellation run; collected + abandoned == submitted. */
export interface CancelBeat {
  jobsSubmitted: number;
  resultsCollected: number;
  jobsAbandoned: number;
  workersStopped: number;
  outcome: string;
}

/** One rigged select race between a reply and a timer. */
export interface TimeoutBeat {
  scenario: string;
  producerDelayMs: number;
  timeoutMs: number;
  /** `"reply"` or `"timer"` — decided by configuration, not luck. */
  winner: string;
  /** `""` when the timer wins — empty is data, not absence. */
  received: string;
}

/** What the runtime reports about itself — observed, not asserted. */
export interface RuntimeFacts {
  gomaxprocs: number;
  goroutinesBefore: number;
  goroutinesDuring: number;
  goroutinesAfter: number;
}

export interface ConcurrencyDemoData {
  setup: PoolSetup;
  fanOut: JobRow[];
  workerCounts: WorkerRow[];
  cancellation: CancelBeat;
  timeouts: TimeoutBeat[];
  runtime: RuntimeFacts;
}
