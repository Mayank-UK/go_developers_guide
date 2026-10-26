/**
 * Domain models for the routing topic's explorer demo — the client-side
 * shape of `GET /api/demo/routing/explorer`. Field names match the
 * server's camelCase wire contract 1:1 (the DTO is the seam where wire
 * and domain may diverge later).
 */

/** How a route pattern's shape is classified (the matcher's vocabulary). */
export type RouteKind = 'exact' | 'wildcard' | 'subtree' | 'trailing-wildcard';

/** The four verdicts the router can return for a probe. */
export type ProbeOutcome = 'matched' | 'method_not_allowed' | 'not_found' | 'redirect';

/** Where a bound parameter came from. */
export type RoutingValueSource = 'path' | 'query';

/** One declared route: the registration line as data. */
export interface RoutingPattern {
  method: string;
  pattern: string;
  kind: RouteKind;
  /**
   * The handler constructor main.go wires to the pattern; OMITTED for
   * exhibit-only patterns (hypothetical registrations used to demonstrate
   * precedence — their missing handler is the marker).
   */
  handler?: string;
}

/** One parameter the router would bind for the handler. */
export interface RoutingValue {
  name: string;
  /** Path values carry one element; query values carry every occurrence. */
  values: string[];
  source: RoutingValueSource;
}

/** One simulated request line and the matcher's verdict on it. */
export interface RoutingProbe {
  method: string;
  /** As written — may carry a `?query` the matcher parses but never routes on. */
  path: string;
  outcome: ProbeOutcome;
  /** 200 / 405 / 404 / 307 — the truthful status for the outcome. */
  status: number;
  matchedPattern?: string;
  pathValues?: RoutingValue[];
  queryValues?: RoutingValue[];
  /** Present only on 405: the mux-computed Allow set, sorted, HEAD included. */
  allow?: string[];
  /** Present only on redirects: the Location the mux would send. */
  redirectTarget?: string;
  /** One-sentence statement of the rule that decided the verdict. */
  reasoning: string;
}

/** A labeled what-if the real table cannot show, resolved against its own patterns. */
export interface RoutingExhibit {
  id: string;
  title: string;
  note: string;
  patterns: RoutingPattern[];
  probes: RoutingProbe[];
}

/** The verified ServeMux rules the matcher implements, as checkable facts. */
export interface RoutingSemantics {
  /** A GET pattern also serves HEAD. */
  getAlsoServesHead: boolean;
  /** A pattern with no method matches every method. */
  methodlessMatchesAll: boolean;
  /** 307 Temporary Redirect — method-preserving — on this Go version. */
  subtreeRedirectStatus: number;
  /** False: an exact pattern never absorbs a slash-appended request. */
  reverseSlashRedirect: boolean;
  /** Matching runs on the escaped path; %2F never splits a segment. */
  matchesEscapedPath: boolean;
  /** PathValue reports the unescaped capture. */
  pathValueIsUnescaped: boolean;
}

export interface RoutingExplorerDemoData {
  /** The guide's real route table, carried as data at authoring time. */
  routes: RoutingPattern[];
  probes: RoutingProbe[];
  exhibits: RoutingExhibit[];
  semantics: RoutingSemantics;
}
