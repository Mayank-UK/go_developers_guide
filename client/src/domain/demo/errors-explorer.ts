/**
 * Domain models for the error-handling topic's explorer demo (demo
 * block id `errors:explorer`) — the client-side shape of
 * `GET /api/demo/errors/explorer`. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and
 * domain may diverge later).
 *
 * One payload carries the taxonomy table — one row per failure kind,
 * with the HTTP status and default message derived from the server's
 * own core tables, the real code that raises the kind, the log line its
 * handling writes, and the trigger that fires it (curl-able, or an
 * honest recipe when the running stack cannot produce that failure) —
 * plus the scenario toggle's panel, the probe panel, and the LIVE walk:
 * the observed status, body, elapsed time, and request id of the probe
 * the explorer just executed under the CURRENT failure injection.
 */

/** How to fire one kind for real — or why only a recipe exists. */
export interface ErrorsTrigger {
  mode: 'live' | 'recipe';
  method: string;
  path: string;
  body?: string;
  prep?: string;
  note: string;
}

/** One failure kind: status, default message, raiser, log, trigger. */
export interface ErrorsKindRow {
  kind: string;
  httpStatus: number;
  defaultMessage: string;
  raisedBy: string;
  loggedAs: string;
  trigger: ErrorsTrigger;
}

/** The scenario sub-panel: the real GET/PATCH endpoints and the modes. */
export interface ErrorsScenarioPanel {
  getEndpoint: string;
  patchEndpoint: string;
  modes: string[];
  note: string;
}

/** The probe panel: the endpoint, the armed latency, the mode notes. */
export interface ErrorsProbePanel {
  probeEndpoint: string;
  latencyMillis: number;
  healthyNote: string;
  injectedNote: string;
  latencyNote: string;
}

/** The live walk: what the probe ACTUALLY answered, just now. */
export interface ErrorsWalkRow {
  injectFailure: string;
  status: number;
  elapsedMillis: number;
  requestID: string;
  body: string;
  note: string;
}

/** The whole explorer payload. */
export interface ErrorsExplorerDemoData {
  kinds: ErrorsKindRow[];
  envelopeNote: string;
  scenario: ErrorsScenarioPanel;
  probe: ErrorsProbePanel;
  walk: ErrorsWalkRow;
  reasoning: string;
}
