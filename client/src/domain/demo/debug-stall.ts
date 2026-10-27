/**
 * Domain models for the debugging topic's stall demo (demo block id
 * `debug:stall`) — the client-side shape of
 * `GET /api/demo/debug/stall`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload's spine is RUNTIME-DERIVED: the server stages a real
 * wedge (one worker of a bounded set blocking forever on a channel
 * receive), captures its OWN goroutine profile while the wedge is
 * live, parses it, and serves the anatomy — WORKLOAD states the
 * choreography, CAPTURE states the provenance (profile name,
 * goroutines seen, cost, attempts, the equivalent curl), STALLED is
 * the wedged goroutine with its frames mapped to source lines, and
 * CONTRAST is the request's own goroutine from the same capture. The
 * wedge is released before the response returns; ENDPOINTS points at
 * the live pprof surface for the reader's own exploration.
 */

/** One stack frame: the function as the runtime printed it, and its file:line. */
export interface DebugStallFrame {
  function: string;
  location: string;
}

/** One goroutine's anatomy as captured: id, state, frames, provenance. */
export interface DebugStallStack {
  id: number;
  state: string;
  wait?: string;
  frames: DebugStallFrame[];
  createdBy?: string;
  createdAt?: string;
}

/** The choreography's facts: worker set size, completions, the wedge itself. */
export interface DebugStallWorkload {
  workers: number;
  healthyDone: number;
  wedge: string;
}

/** The capture's provenance: what was taken, what it cost, how to read it live. */
export interface DebugStallCapture {
  profile: string;
  goroutinesSeen: number;
  captureMs: number;
  captureTries: number;
  curl: string;
}

/** One live pprof endpoint the server mounts under /internal/. */
export interface DebugStallEndpoint {
  path: string;
  profile: string;
  note: string;
}

/** The whole stall payload. */
export interface DebugStallDemoData {
  mode: string;
  workload: DebugStallWorkload;
  capture: DebugStallCapture;
  stalled: DebugStallStack;
  contrast: DebugStallStack;
  endpoints: DebugStallEndpoint[];
  reasoning: string;
}
