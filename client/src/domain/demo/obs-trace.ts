/**
 * Domain models for the observability topic's trace demo (demo block
 * id `obs:trace`) — the client-side shape of
 * `GET /api/demo/obs/trace`. Field names match the server's camelCase
 * wire contract 1:1.
 *
 * One header became one tree: the ACCEPTED panel shows the inbound
 * traceparent (yours when the request carried one, minted when it did
 * not), the parsed fields, and the response echo carrying the server's
 * own span id. The TREE is the span walk — middleware → thin handler →
 * use case/port → adapter — with the graft point: the root's parent is
 * the INBOUND remote span id. The MALFORMED table parses the spec's
 * hostile headers for real, each row carrying the parser's actual
 * rejection reason and the never-fails-the-request policy.
 */

/** The acceptance panel: what arrived, what parsed, what echoed. */
export interface ObsTraceAccepted {
  inbound: string;
  inboundOrigin: string;
  version: string;
  traceId: string;
  parentId: string;
  flags: string;
  sampled: boolean;
  tracestate?: string;
  responseEcho: string;
  note: string;
}

/** One node of the rendered span tree, pre-order with depths. */
export interface ObsSpanRow {
  name: string;
  layer: string;
  spanId: string;
  parentId: string;
  traceId: string;
  depth: number;
  startOffsetMs: number;
  durationMs: number;
  note: string;
}

/** One hostile header, its real rejection, and the handling policy. */
export interface ObsMalformedRow {
  header: string;
  problem: string;
  handledAs: string;
}

/** The whole trace round-trip payload. */
export interface ObsTraceDemoData {
  accepted: ObsTraceAccepted;
  tree: ObsSpanRow[];
  malformed: ObsMalformedRow[];
  sdk: string;
  reasoning: string;
}
