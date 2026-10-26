/**
 * Failure — clean, predictable domain-level error states the UI understands,
 * mirroring the express guide's client taxonomy (react-guide `Failure`),
 * carried over unchanged by the spring guide.
 *
 * A Failure has:
 *  - a user-friendly `message`,
 *  - a `kind` + `severity` the UI can branch on,
 *  - the machine diagnostics preserved from the transport layer: `code` (the
 *    error envelope's machine code, e.g. 'not_found'), `status` (HTTP status;
 *    0 = the request never completed), and `requestId` (the server's request
 *    correlation id, surfaced next to the diagnosis so a failure can be traced
 *    in the server logs).
 *
 * Severity decides HOW the UI surfaces a failure:
 *  - Info / Warning → inline (a chip, a muted note); the screen still works.
 *  - Critical       → full-screen error state (ErrorPanel); the screen has no
 *                     data to render. Every transport failure this client
 *                     guards is Critical — the split exists so future
 *                     non-blocking failures (stale cache, degraded feature)
 *                     have a place to land without reworking the taxonomy.
 *
 * `FailureSeverity` is a const object + literal union (not an `enum`).
 */

export const FailureSeverity = {
  /** Gentle, dismissible (inline note). Data is still usable. */
  Info: 'info',
  /** Non-blocking notice (inline). e.g. showing stale cache. */
  Warning: 'warning',
  /** Blocking — the screen has no data and must show a full-screen error. */
  Critical: 'critical',
} as const;

export type FailureSeverity = (typeof FailureSeverity)[keyof typeof FailureSeverity];

export type FailureKind = 'network' | 'server' | 'unknown';

/** Machine diagnostics preserved from the transport layer for the UI. */
export interface FailureDiagnostics {
  /** Machine code from the error envelope ('network_error', 'not_found', …). */
  code: string;
  /** HTTP status; 0 means the request never completed. */
  status: number;
  /** Server request correlation id from the error envelope, when present. */
  requestId?: string;
}

export abstract class Failure {
  abstract readonly kind: FailureKind;
  abstract readonly severity: FailureSeverity;
  readonly code: string;
  readonly status: number;
  readonly requestId?: string;
  private readonly rawMessage: string;

  constructor(message: string, diagnostics: FailureDiagnostics) {
    this.rawMessage = message;
    this.code = diagnostics.code;
    this.status = diagnostics.status;
    this.requestId = diagnostics.requestId;
  }

  get message(): string {
    return this.rawMessage;
  }
}

/** The fetch never completed: offline, DNS, CORS, connection refused. */
export class NetworkFailure extends Failure {
  readonly kind = 'network' as const;
  readonly severity = FailureSeverity.Critical;
  constructor(
    message = 'Could not reach the server.',
    diagnostics: FailureDiagnostics = { code: 'network_error', status: 0 },
  ) {
    super(message, diagnostics);
  }
}

/** The server answered with a non-2xx status (error envelope) or a bad body. */
export class ServerFailure extends Failure {
  readonly kind = 'server' as const;
  readonly severity = FailureSeverity.Critical;
  constructor(
    message = 'Server error',
    diagnostics: FailureDiagnostics = { code: 'unknown_error', status: 0 },
  ) {
    super(message, diagnostics);
  }
}

/** Anything the guards could not classify — still a typed, renderable state. */
export class UnknownFailure extends Failure {
  readonly kind = 'unknown' as const;
  readonly severity = FailureSeverity.Critical;
  constructor(
    message = 'Something went wrong',
    diagnostics: FailureDiagnostics = { code: 'unknown_error', status: 0 },
  ) {
    super(message, diagnostics);
  }
}
