/**
 * Domain models for the observability topic's slog viewer demo (demo
 * block id `obs:viewer`) — the client-side shape of
 * `GET /api/demo/obs/viewer`. Field names match the server's camelCase
 * wire contract 1:1 (the DTO is the seam where wire and domain may
 * diverge later).
 *
 * Three panels, one lesson: the log line is data. The ANATOMY dissects
 * one real http_request line field by field. The LEVELS matrix shows
 * which severity lines survive which GDG_LOG_LEVEL configuration —
 * computed server-side by really gating statements through the handler.
 * The REPLAY walks a 200 and a 404 through the production middleware
 * chain in-process and captures the lines verbatim, proving the
 * request-id correlation that turns thousands of lines into one story.
 */

/** One dissected field of the anatomy line. */
export interface ObsViewerFieldRow {
  key: string;
  value: string;
  note: string;
}

/** The dissected access line. */
export interface ObsViewerAnatomy {
  line: string;
  fields: ObsViewerFieldRow[];
  note: string;
}

/** One severity level row of the gate matrix. */
export interface ObsViewerLevelRow {
  level: string;
  survivesAt: string[];
  emittedBy: string;
  note: string;
}

/** The level matrix panel. */
export interface ObsViewerLevels {
  rows: ObsViewerLevelRow[];
  gate: string;
  note: string;
}

/** One replayed request the handler walked through the chain. */
export interface ObsViewerReplayedRequest {
  label: string;
  request: string;
  requestId: string;
  status: number;
}

/** One captured log line, parsed to its correlation fields. */
export interface ObsViewerReplayLine {
  source: string;
  level?: string;
  msg?: string;
  requestId?: string;
  raw: string;
}

/** The correlation panel. */
export interface ObsViewerReplay {
  requests: ObsViewerReplayedRequest[];
  lines: ObsViewerReplayLine[];
  note: string;
}

/** The whole slog viewer payload. */
export interface ObsViewerDemoData {
  anatomy: ObsViewerAnatomy;
  levels: ObsViewerLevels;
  replay: ObsViewerReplay;
  reasoning: string;
}
