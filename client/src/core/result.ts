import type { Failure } from './error-handling/failures';

/**
 * Result<T> — the domain return type for anything that can fail in an
 * expected, recoverable way. Mirrors the react/express/spring guides'
 * `Result<T>`; the Go server's counterpart is its plain `(value, error)`
 * returns shaped by `server/internal/core/errors.go`'s Kind taxonomy —
 * the same taxonomy `core/error-handling/failures.ts` types for this
 * side. A discriminated union on `ok` so switch/narrowing works
 * ergonomically in TypeScript.
 *
 * - `Success` carries a value and an *optional* `warning` (a non-blocking
 *   Failure, e.g. "showed stale cache because the network failed").
 * - `Failed` carries the blocking Failure (core/error-handling/failures.ts —
 *   the typed taxonomy with kind, severity, code, status, and requestId).
 *
 * Repositories guarantee a Result for EVERY outcome (they never throw); views
 * and use cases branch on `ok` alone.
 */
export type Result<T> = Success<T> | Failed;

export interface Success<T> {
  ok: true;
  value: T;
  warning?: Failure;
}

export interface Failed {
  ok: false;
  failure: Failure;
}

export const success = <T>(value: T, warning?: Failure): Success<T> => ({
  ok: true,
  value,
  warning,
});

export const failed = (failure: Failure): Failed => ({ ok: false, failure });
