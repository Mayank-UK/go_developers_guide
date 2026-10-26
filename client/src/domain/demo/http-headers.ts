/**
 * Domain models for the http-web topic's headers demo — the client-side
 * shape of `GET /api/demo/http/headers`. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and domain
 * may diverge later).
 */

/** Header scope values the server classifies (RFC 9110's split). */
export type HeaderScope = 'end-to-end' | 'hop-by-hop';

/** Header role values the server classifies (what a header is FOR). */
export type HeaderRole =
  | 'negotiation'
  | 'cors'
  | 'security'
  | 'identity'
  | 'extension'
  | 'transport'
  | 'other';

/**
 * What the server shows instead of a credential bearer's value — the
 * wire contract string (Go's demos.RedactedHeaderValue). An echo must
 * never become a leak.
 */
export const REDACTED_HEADER_VALUE = '[value withheld]';

export interface EchoedHeader {
  /** Canonical header name, e.g. `Accept-Encoding` or `X-Custom`. */
  name: string;
  /** Joined values; credential bearers show `REDACTED_HEADER_VALUE`. */
  value: string;
  /** End-to-end vs hop-by-hop — RFC 9110 §7.6.1's split. */
  scope: HeaderScope;
  /** What the header is for: negotiation, cors, security, … */
  role: HeaderRole;
  /** One-line teaching note; absent when there is nothing to say. */
  note?: string;
}

export interface HttpHeadersDemoData {
  method: string;
  path: string;
  /** Raw query string (no leading `?`); omitted when the request had none. */
  query?: string;
  /** The negotiated protocol for this request, e.g. `HTTP/1.1`. */
  proto: string;
  /** The request id the guide's middleware honored or minted. */
  requestID: string;
  /** The inbound Content-Type; omitted for a body-less request. */
  contentType?: string;
  /** Every inbound header, classified — base teaching order, then sorted. */
  headers: EchoedHeader[];
  /** The RFC 9110 §7.6.1 hop-by-hop reference list (+ keep-alive), sorted. */
  hopByHop: string[];
}
