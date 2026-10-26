/**
 * Domain models for the go-language topic's runtime-types demo — the
 * client-side shape of `GET /api/demo/go/runtime-types`. Field names match
 * the server's camelCase wire contract 1:1 (the DTO is the seam where wire
 * and domain may diverge later).
 */

export interface TypeRow {
  /** The source expression reflect looked at, e.g. `content.Topic{}`. */
  expression: string;
  /** reflect.TypeOf(v).String() — the dynamic type's printed name. */
  type: string;
  /** The reflect.Kind: struct, func, slice, map, string, int, ptr, … */
  kind: string;
  /** The type's package path; empty (`""`) for predeclared types. */
  pkgPath: string;
  /** The type's size in bytes, as the runtime lays it out. */
  sizeBytes: number;
}

export interface FieldRow {
  name: string;
  type: string;
  /** The raw `json:"…"` tag value — wire naming as runtime metadata. */
  jsonTag: string;
}

export interface AssertionRow {
  /** The asserted expression, e.g. `blocks[2].(type)`. */
  expression: string;
  dynamicType: string;
  /** Whether the assertion matched (a comma-ok miss reports false). */
  ok: boolean;
  /** The id the type switch resolved; `""` on a miss. */
  blockID: string;
}

export interface ZeroValueRow {
  /** The declaration as written, e.g. `var present bool`. */
  declaration: string;
  type: string;
  /** How Go source writes the zero: `""`, `false`, `{}`, `nil or 0`. */
  display: string;
  /** reflect.Value.IsZero() confirmation. */
  isZero: boolean;
}

export interface EnforcementRun {
  function: string;
  signature: string;
  /** The wrong-typed call the compiler rejects. */
  call: string;
  /** The compiler's own verdict, quoted verbatim from `go build`. */
  compilerVerdict: string;
  /** What runtime enforcement remains: none — the call never builds. */
  runtimeOutcome: string;
}

export interface BoundaryCheck {
  decodeTarget: string;
  badInput: string;
  outcome: string;
  errorType: string;
  message: string;
}

export interface RuntimeTypesDemoData {
  inspections: TypeRow[];
  fields: FieldRow[];
  assertions: AssertionRow[];
  zeroValues: ZeroValueRow[];
  enforcement: EnforcementRun;
  boundary: BoundaryCheck;
}
