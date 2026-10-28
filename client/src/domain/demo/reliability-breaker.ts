/**
 * Domain models for the reliability topic's breaker demo (demo block
 * id `reliability:breaker`) — the client-side shape of
 * `GET /api/demo/reliability/breaker`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload is LIVE-STATE: the server walks a bounded series of
 * downstream calls through a circuit breaker that PERSISTS for the
 * process lifetime (closed/open/half-open, failure threshold,
 * cooldown, single-probe semantics), where the dependency honors the
 * SHARED scenario panel's injectFailure enum (none|error|latency) and
 * respects context deadlines. SCENARIO names the armed mode and the
 * real toggle endpoints; BREAKER carries the persistent state with
 * its age, config, counters, and transition ring; WALK is this
 * request's attempt-by-attempt timeline; RETRY is a bounded
 * exponential-backoff series beside it. Every elapsed/backoff figure
 * was MEASURED around the calls that just happened — nothing on the
 * wire is invented.
 */

/** The scenario toggle the demo's dependency honors. */
export interface ReliabilityScenarioPanel {
  mode: string;
  getEndpoint: string;
  patchEndpoint: string;
  modes: string[];
  note: string;
}

/** The config the machinery actually runs with (policy, not results). */
export interface ReliabilityConfigPanel {
  failureThreshold: number;
  cooldownMs: number;
  callTimeoutMs: number;
  walkAttempts: number;
  halfOpenProbes: number;
  retryAttempts: number;
  retryBaseDelayMs: number;
  retryJitterMs: number;
}

/** Process-lifetime totals since boot. */
export interface ReliabilityCounters {
  allowed: number;
  fastFailed: number;
  succeeded: number;
  failed: number;
}

/** One observed state change of the persistent breaker. */
export interface ReliabilityTransitionView {
  at: string;
  from: string;
  to: string;
  cause: string;
}

/** The persistent breaker's observation at request time. */
export interface ReliabilityBreakerPanel {
  state: string;
  stateAgeMs: number;
  config: ReliabilityConfigPanel;
  counters: ReliabilityCounters;
  probeInFlight: boolean;
  transitions: ReliabilityTransitionView[];
}

/** One attempt of this request's walk through the breaker. */
export interface ReliabilityAttemptView {
  index: number;
  stateBefore: string;
  action: string;
  outcome: string;
  detail?: string;
  elapsedMs: number;
  stateAfter: string;
}

/** This request's walk, with its derived note. */
export interface ReliabilityWalkPanel {
  mode: string;
  rows: ReliabilityAttemptView[];
  note: string;
}

/** One attempt of the retry-with-backoff series. */
export interface ReliabilityRetryRowView {
  attempt: number;
  outcome: string;
  detail?: string;
  elapsedMs: number;
  backoffMs: number;
}

/** The retry series: policy-bounded attempts with measured waits. */
export interface ReliabilityRetryPanel {
  mode: string;
  succeeded: boolean;
  rows: ReliabilityRetryRowView[];
  totalElapsedMs: number;
  note: string;
}

/** The whole reliability breaker payload. */
export interface ReliabilityBreakerDemoData {
  mode: string;
  scenario: ReliabilityScenarioPanel;
  breaker: ReliabilityBreakerPanel;
  walk: ReliabilityWalkPanel;
  retry: ReliabilityRetryPanel;
  reasoning: string;
}
