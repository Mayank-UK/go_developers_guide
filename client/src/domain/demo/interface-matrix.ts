/**
 * Domain models for the interfaces-and-composition topic's satisfaction
 * matrix demo — the client-side shape of `GET /api/demo/iface/matrix`.
 * Field names match the server's camelCase wire contract 1:1 (the DTO is
 * the seam where wire and domain may diverge later).
 */

/** One column of the matrix: an interface and the single method that defines it. */
export interface InterfaceSpec {
  /** The interface's qualified name, e.g. `io.Reader`. */
  name: string;
  /** The defining method rendered from the real reflect type, e.g. `Read([]byte) (int, error)`. */
  method: string;
  /** Where the interface lives: `io`, `net/http`, `builtin`, or this repo's content package path. */
  pkg: string;
  /** One line on why the interface matters. */
  note: string;
}

/** One row × column verdict — `satisfied` is computed by the server from the real method set. */
export interface MatrixCell {
  /** The column's interface name (matches an InterfaceSpec.name). */
  iface: string;
  /** Whether the row's type satisfies the interface, per reflect's Implements. */
  satisfied: boolean;
}

/** One concrete type under inspection: its identity, method set, and verdicts. */
export interface SatisfactionRow {
  /** The source expression inspected, e.g. `*core.Error`. */
  expression: string;
  /** The dynamic type's printed name. */
  type: string;
  /** The reflect kind: struct, ptr, func, int64, … */
  kind: string;
  /** The exported method set, sorted — empty is data (sealed unexported method, value receiver gap). */
  methods: string[];
  /** One verdict per column, in the payload's column order. */
  cells: MatrixCell[];
}

/** One composition exhibit: satisfaction purely through promoted methods. */
export interface EmbeddingRow {
  /** The declaration as written, e.g. `type delegatingHandler struct { http.Handler }`. */
  expression: string;
  /** What is embedded, concrete type or interface. */
  embeds: string;
  /** The exhibit type's (or target's) printed name. */
  type: string;
  /** The interface the exhibit satisfies. */
  iface: string;
  /** Whether it satisfies — always computed, never authored. */
  satisfied: boolean;
  /** Why this exhibit teaches: the promotion story. */
  note: string;
}

/** One input through the demo's type switch: which case matched and why. */
export interface SwitchCaseRow {
  /** The input expression, e.g. `*bytes.Buffer`. */
  input: string;
  /** The case that matched, e.g. `case io.Reader`. */
  matchedCase: string;
  /** What classify returned for this input. */
  result: string;
  /** Present when the row carries the order-of-cases lesson. */
  note?: string;
}

/** One `var _ Iface = Type{}` line and the matrix cell it pins at build time. */
export interface CompileProof {
  /** The source line, verbatim from the demo module. */
  line: string;
  /** The cell it pins, e.g. `error × *core.Error`. */
  pins: string;
}

/** The full satisfaction-matrix payload. */
export interface InterfaceMatrixDemoData {
  /** The five columns, method signatures rendered from the real types. */
  interfaces: InterfaceSpec[];
  /** The type axis: eleven real guide and stdlib types with live verdicts. */
  rows: SatisfactionRow[];
  /** Three promotion exhibits. */
  embeddings: EmbeddingRow[];
  /** Six inputs through the type switch, first match wins. */
  typeSwitch: SwitchCaseRow[];
  /** The compile-time proofs as data, cross-checked by the server's tests. */
  compileProofs: CompileProof[];
}
