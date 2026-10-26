/**
 * Domain models for the request-response topic's inspector demo — the
 * client-side shape of `GET /api/demo/reqres/inspector`. Field names
 * match the server's camelCase wire contract 1:1 (the DTO is the seam
 * where wire and domain may diverge later).
 */

/** Verdict values the decoder discipline can hand a body. */
export type ReqresDecodeOutcome = 'accepted' | 'rejected';

/** Live-beat outcomes: a body-less GET reports `none`. */
export type ReqresLiveOutcome = 'accepted' | 'rejected' | 'none';

/** One exhibit body's verdict: input verbatim, outcome, error, decoded state. */
export interface ReqresDecodeExhibit {
  /** What this exhibit teaches, e.g. `truncated mid-array`. */
  label: string;
  /** The exhibit body exactly as the decoder received it. */
  body: string;
  outcome: ReqresDecodeOutcome;
  /** The decoder's first error, verbatim; absent when accepted. */
  error?: string;
  /** The error's concrete Go type, e.g. `*json.UnmarshalTypeError`. */
  errorType?: string;
  /** The offending field when the error names one. */
  field?: string;
  /** Truncation flag — checked with errors.Is on the server, never strings. */
  unexpectedEOF?: boolean;
  /**
   * The decoded (possibly partial) state as Go source would write it;
   * present-but-empty is DATA — a truncated body decodes nothing.
   */
  decoded: string;
}

/** The live request's own facts, extracted by the thin handler. */
export interface ReqresInspectorRequest {
  method: string;
  path: string;
  /** Inbound Content-Type; omitted for a body-less request. */
  contentType?: string;
  /** Bytes that actually arrived. */
  bodyBytes: number;
  /** The cap the handler enforced (4096). */
  bodyLimit: number;
  /** Set when the body blew past the cap and arrived truncated. */
  bodyLimitExceeded?: boolean;
}

/** The decoder discipline stated as rules that render beside the verdicts. */
export interface ReqresDisciplineFacts {
  decoder: string;
  unknownFields: string;
  trailing: string;
  firstError: string;
}

/** The same discipline applied to the body THIS request carried. */
export interface ReqresLiveBody {
  /** Whether a body arrived at all; a plain GET carries none. */
  sent: boolean;
  /** The body bytes as received; omitted when nothing was sent. */
  body?: string;
  outcome: ReqresLiveOutcome;
  error?: string;
  errorType?: string;
  field?: string;
  unexpectedEOF?: boolean;
  /** Decoded state; omitted when there was nothing to decode. */
  decoded?: string;
}

export interface ReqresInspectorDemoData {
  request: ReqresInspectorRequest;
  discipline: ReqresDisciplineFacts;
  exhibits: ReqresDecodeExhibit[];
  liveBody: ReqresLiveBody;
}
