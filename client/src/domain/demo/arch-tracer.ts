/**
 * Domain models for the architecture topic's tracer demo (demo block id
 * `arch:tracer`) — the client-side shape of `GET /api/demo/arch/tracer`.
 * Field names match the server's camelCase wire contract 1:1 (the DTO is
 * the seam where wire and domain may diverge later).
 *
 * One payload, two panels. The TRACE is a real posts-list request the
 * server walked through its actual layers with recorders at the seams:
 * middleware → thin handler → use case → the adapter the scenario
 * picked, every hop labeled by the file and symbol that ran and timed
 * inclusively at its seam. The VIOLATION panel is the dependency rule's
 * negative space — what a handler importing the data layer would look
 * like, carried as clearly-labeled non-compiling data.
 */

/** The layer names the timeline discriminates on. */
export type ArchLayer = 'middleware' | 'presentation' | 'domain' | 'data';

/** One stop on the traced timeline, outermost first. */
export interface ArchTraceHop {
  order: number;
  layer: ArchLayer;
  file: string;
  symbol: string;
  note: string;
  durationMs: number;
}

/** The traced request's timeline and what it answered. */
export interface ArchTracePanel {
  request: string;
  dataMode: 'fake' | 'live';
  leaf: string;
  flipNote: string;
  hops: ArchTraceHop[];
  status: number;
  items: number;
  elapsedMs: number;
  requestId: string;
  note: string;
}

/** The dependency-rule counterexample panel. */
export interface ArchViolationPanel {
  label: string;
  snippet: string;
  why: string;
  caughtBy: string;
  honest: string;
}

/** The whole architecture tracer payload. */
export interface ArchTracerDemoData {
  trace: ArchTracePanel;
  violation: ArchViolationPanel;
  reasoning: string;
}
