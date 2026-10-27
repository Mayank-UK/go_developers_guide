/**
 * Domain models for the authorization topic's matrix demo (demo block id
 * `authz:matrix`) — the roles × scopes × ownership decision matrix the
 * server evaluates in one request through its real policy evaluator. The
 * wire shape is the story shape the authentication demos established
 * (mechanism, beats with verdict colors and label/value facts, derived
 * proofs, closing reasoning); this topic owns its own types so the slice
 * stays self-contained and the DTO remains the seam where wire and domain
 * may diverge later.
 */

/** One label/value pair — roster grants, subject rows, cell citations. */
export interface AuthzMatrixFact {
  label: string;
  value: string;
}

/** One beat of the matrix story: the teaching moment and its verdicts. */
export interface AuthzMatrixBeat {
  beat: string;
  detail: string;
  /** 'ok' | 'rejected' | 'alarm' | 'info' — the client colors, never interprets. */
  verdict: string;
  facts: AuthzMatrixFact[];
}

/** The whole matrix payload GET /api/demo/authz/matrix serves. */
export interface AuthzMatrixDemoData {
  mechanism: string;
  beats: AuthzMatrixBeat[];
  proofs: AuthzMatrixFact[];
  reasoning: string;
}
