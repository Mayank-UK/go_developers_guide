/**
 * Domain models for the middleware topic's pipeline demo — the
 * client-side shape of `GET /api/demo/middleware/pipeline`. Field names
 * match the server's camelCase wire contract 1:1 (the DTO is the seam
 * where wire and domain may diverge later).
 */

/** Where in the onion one trace beat fired: descending, the handler, ascending. */
export type TracePhase = 'before' | 'handler' | 'after';

/** One row of the exhibit chain, outermost first (position 0 sees the request first). */
export interface ChainLayer {
  /** Layer name, e.g. `request-id` or `terminal handler`. */
  name: string;
  /** 0 = outermost — the layer whose code runs first on the way in. */
  position: number;
  /** What the layer does, one line. */
  role: string;
  /** The production counterpart this row mirrors, e.g. `presentation.RequestID`. */
  mirrors: string;
}

/**
 * One beat of the traced run; `sequence` is the chronological truth.
 * `requestIDSeen` may legitimately be `''` — the beat fired before the
 * request-id layer had assigned anything (empty is DATA, not absence).
 */
export interface TraceEvent {
  layer: string;
  phase: TracePhase;
  sequence: number;
  requestIDSeen: string;
  note: string;
}

/** The request-id propagation proof: one id in, the same id everywhere. */
export interface RequestIDBeat {
  /** The header the contract rides on: `x-request-id`. */
  headerName: string;
  /** The id the traced request carried inbound (so the run is deterministic). */
  inbound: string;
  /** `layer (phase)` for every beat that observed a non-empty id. */
  seenAtLayers: string[];
  /** The id echoed on the traced response's header. */
  responseHeader: string;
  /** True when every layer past the first saw the same id and it echoed. */
  sameIDEverywhere: boolean;
}

/** The recovered-panic run's report — a deliberate bug, converted. */
export interface PanicRecoveryBeat {
  path: string;
  /** The panic value, e.g. `assignment to entry in nil map`. */
  cause: string;
  /** Always 500 — the envelope the recover layer wrote. */
  status: number;
  /** The 500 envelope body verbatim (JSON + trailing newline). */
  body: string;
  /** The captured slog line (time field removed); cites the same request id. */
  loggedLine: string;
  /** True when the 500 response still carried the request-id header. */
  requestIDKept: boolean;
  /** The panic run's own trace — the unwind order survives the panic. */
  trace: TraceEvent[];
}

/** The http.TimeoutHandler run's report — shape, never wall-clock timings. */
export interface TimeoutBeat {
  /** Always `http.TimeoutHandler`. */
  wrapper: string;
  /** The wrapper's budget in milliseconds (30 in the pinned run). */
  budgetMs: number;
  /** The slow handler's sleep in milliseconds (120 in the pinned run). */
  handlerSleepMs: number;
  /** The wrapper's own answer when the budget loses (503). */
  observedStatus: number;
  /** The wrapper's canned body, verbatim. */
  body: string;
  /** True when the abandoned handler goroutine finished within the bounded join. */
  slowHandlerFinished: boolean;
  /** Prose describing the mechanics. */
  shape: string;
}

export interface MiddlewarePipelineDemoData {
  /** The exhibit chain's rows, outermost first. */
  chain: ChainLayer[];
  /** The traced run's beats in chronological order. */
  trace: TraceEvent[];
  /** Layer names in request order — outermost first, the handler last. */
  requestSideOrder: string[];
  /** The same names as the response unwinds — innermost first. */
  responseSideOrder: string[];
  requestID: RequestIDBeat;
  panicRecovery: PanicRecoveryBeat;
  timeoutBeat: TimeoutBeat;
  /** The server's own summary of what the trace proves. */
  reasoning: string;
}
