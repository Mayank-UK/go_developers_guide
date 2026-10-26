/**
 * Scenario domain models — the pure shape of the app's single runtime demo
 * toggle: which data mode demos answer from and which failure injection is
 * armed. They know nothing about JSON or HTTP; the data layer validates wire
 * payloads into these shapes (see data/scenario/scenario-dto.ts).
 *
 * Const-object + literal-union style throughout (no enums — erasable syntax):
 * the const objects exist so code that needs the enum *values* as data (the
 * DTO parser, the panel's selects) can compare without importing string
 * literals ad hoc.
 */

/** Which backing store the demos answer from (fake data ships from day one; the live stores wire up with the data phase). */
export const DataMode = {
  Fake: 'fake',
  Live: 'live',
} as const;

export type DataMode = (typeof DataMode)[keyof typeof DataMode];

/**
 * Which failure path the demos inject — the family's three-value enum, not a
 * bool: the teaching distinguishes error injection (error) from latency
 * injection (latency), and the wire carries these exact strings.
 */
export const InjectFailure = {
  None: 'none',
  Error: 'error',
  Latency: 'latency',
} as const;

export type InjectFailure = (typeof InjectFailure)[keyof typeof InjectFailure];

/** The store's immutable snapshot; defaults are fake data, no injection. */
export interface ScenarioState {
  dataMode: DataMode;
  injectFailure: InjectFailure;
}

/**
 * A partial update: absent keys mean "unchanged". A patch carrying only
 * `injectFailure` must never imply anything about `dataMode`.
 */
export interface ScenarioPatch {
  dataMode?: DataMode;
  injectFailure?: InjectFailure;
}
