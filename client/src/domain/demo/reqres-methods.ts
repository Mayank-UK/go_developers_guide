/**
 * Domain models for the request-response topic's methods demo — the
 * client-side shape of `GET /api/demo/reqres/methods`. Field names match
 * the server's camelCase wire contract 1:1 (the DTO is the seam where
 * wire and domain may diverge later).
 */

/** One response header exactly as the recorder observed it. */
export interface ReqresObservedHeader {
  name: string;
  value: string;
}

/** One method's observed outcome from the in-process exhibit run. */
export interface ReqresMethodRow {
  method: string;
  /** The status the exhibit wrote, read back off the recorder. */
  status: number;
  /** The stdlib's phrase for the status (http.StatusText). */
  statusText: string;
  /** Selected observed headers (Location, Set-Cookie, Allow, …). */
  headers: ReqresObservedHeader[];
  /** The one-line body-shape rule this row teaches. */
  bodyShape: string;
  /**
   * The exact body bytes; empty is DATA — 204/HEAD/OPTIONS rows carry
   * no body by protocol rule.
   */
  body: string;
}

/** One parsed Server-Sent-Event block: its name and data payload. */
export interface ReqresSSEEvent {
  name: string;
  data: string;
}

/** The streaming beat reported structurally — no timing values. */
export interface ReqresSSEBeat {
  contentType: string;
  events: ReqresSSEEvent[];
  /** Observed through a counting Flush wrapper, not asserted. */
  flushes: number;
  framing: string;
}

export interface ReqresMethodsDemoData {
  /** The fictional resource path the exhibit models. */
  resource: string;
  /** The capability statement: the methods the exhibit serves. */
  allow: string;
  /** Eight rows: seven served methods plus a truthful 405 for TRACE. */
  methods: ReqresMethodRow[];
  sse: ReqresSSEBeat;
}
