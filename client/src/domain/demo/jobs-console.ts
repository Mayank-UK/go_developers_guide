/**
 * Domain models for the background-jobs topic's console demo (demo
 * block id `jobs:console`) — the client-side shape of
 * `GET /api/demo/jobs/console`. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and
 * domain may diverge later).
 *
 * One payload walks the whole choreography against the RUNNING stack
 * Redis and the hosted worker: the weighted queues, each task's
 * attempt ladder (the flaky task's fail-fail-succeed IS the backoff
 * story, observed), the beats in execution order (enqueue, delayed,
 * retry, archive, replay, cron, observe, cleanup), the cron entry's
 * spec and ticks, the two observability numbers at their worst and at
 * rest, and the five derived proofs.
 */

/** One hosted queue and its weight (proportional polling share). */
export interface JobsQueue {
  name: string;
  weight: number;
}

/** One attempt as the ledger recorded it: 1-based number + outcome. */
export interface JobsAttempt {
  attempt: number;
  outcome: string;
}

/** One task's whole story: identity, routing, budget, ladder, verdict. */
export interface JobsTask {
  id: string;
  type: string;
  queue: string;
  maxRetry: number;
  attempts: JobsAttempt[];
  verdict: string;
}

/** One beat of the story: what was done, the commands, the outcome. */
export interface JobsConsoleBeat {
  beat: string;
  commands: string[];
  outcome: string;
}

/** The scheduler entry's spec and its observed fires. */
export interface JobsCron {
  spec: string;
  ticks: number;
}

/** The two observability numbers, at their worst and at rest. */
export interface JobsObservability {
  peakDepth: number;
  peakOldestAgeMs: number;
  finalDepth: number;
  finalOldestAgeMs: number;
}

/** The story's five derived verdicts. */
export interface JobsConsoleProofs {
  retry: string;
  archive: string;
  replay: string;
  cron: string;
  observability: string;
}

/** The whole console-story payload. */
export interface JobsConsoleDemoData {
  namespace: string;
  queues: JobsQueue[];
  commands: string[];
  tasks: JobsTask[];
  beats: JobsConsoleBeat[];
  cron: JobsCron;
  observability: JobsObservability;
  proofs: JobsConsoleProofs;
  reasoning: string;
}
