/**
 * Domain models for the webhooks topic's delivery-simulator demo — the
 * client-side shape of `GET /api/demo/api/webhook-simulator`. Field names
 * match the server's camelCase wire contract 1:1 (the DTO is the seam
 * where wire and domain may diverge later).
 */

/** The retry ladder's ruling on one attempt. */
export type AttemptRuling = 'retry-scheduled' | 'delivered' | 'dead-lettered';

/** A delivery's final outcome after walking the retry ladder. */
export type DeliveryOutcome = 'delivered' | 'dead-lettered';

/** The fixed knob set the whole simulation runs under. */
export interface SimulatorConfig {
  /** Always `HMAC-SHA256` — the stdlib's crypto the server really ran. */
  algorithm: string;
  /** The header the signature rides in, e.g. `X-Guide-Signature`. */
  signatureHeader: string;
  /** The header's documented shape, e.g. `t={timestamp},n={nonce},v1={…}`. */
  headerFormat: string;
  /** The canonical message the HMAC covers, e.g. `{timestamp}.{nonce}.{body}`. */
  messageFormat: string;
  /** How old a signed timestamp may be and still count as fresh. */
  toleranceSeconds: number;
  /** Attempts before the ladder gives up into the dead-letter queue. */
  maxAttempts: number;
  /** The first retry's wait; each later retry doubles it. */
  baseDelaySeconds: number;
  /** The ceiling doubling never passes. */
  maxDelaySeconds: number;
  /** Human-readable formula, e.g. `base × 2^(attempt-1), capped at max`. */
  delayPattern: string;
}

/** What the sender computed and put on the wire for one delivery. */
export interface IssuedSignature {
  /** RFC 3339 signing time — the receiver's clock check starts here. */
  timestamp: string;
  /** Distinguishes two deliveries signed in the same second. */
  nonce: string;
  /** Hex-encoded HMAC-SHA256 over the canonical message. */
  signature: string;
  /** The full header line as it would ride in `X-Guide-Signature`. */
  headerLine: string;
}

/** One verification verdict: what the receiver's real gate answered. */
export interface SignatureCase {
  id: string;
  description: string;
  /** The fixed receiver clock this case ran at (RFC 3339). */
  receiverClock: string;
  /** The presented signature, truncated for display. */
  presented: string;
  /** The gate's verdict: `true` only for the honest delivery. */
  verified: boolean;
  /** The gate's error message; `''` when verified — empty is DATA. */
  reason: string;
  /** One-line teaching note on why the verdict is what it is. */
  note: string;
}

/** The sender's signing material plus the verdicts it earned. */
export interface SigningExhibit {
  /** Where the secret lives (hint form — never the secret itself). */
  secretHint: string;
  /** The canonical signed body — the exact bytes the HMAC covers. */
  body: string;
  issued: IssuedSignature;
  cases: SignatureCase[];
}

/** One attempt in a delivery's ledger. */
export interface AttemptRow {
  /** 1-based attempt number. */
  attempt: number;
  /** Computed backoff waited BEFORE this attempt (data — nothing slept). */
  waitSeconds: number;
  /** The endpoint's simulated HTTP status. */
  status: number;
  ruling: AttemptRuling;
}

/** The DLQ record a delivery lands in when the ladder exhausts. */
export interface DeadLetter {
  queue: string;
  reason: string;
  /** The operational door out: fix the receiver, replay the delivery. */
  replayable: boolean;
}

/** One delivery walked through the retry ladder. */
export interface DeliveryLedger {
  deliveryId: string;
  event: string;
  endpoint: string;
  attempts: AttemptRow[];
  outcome: DeliveryOutcome;
  attemptsUsed: number;
  /** Total computed backoff across the ladder (0s before attempt 1). */
  totalWaitSeconds: number;
  /** Present only when the ladder exhausted — omitted, never null. */
  deadLetter?: DeadLetter;
}

/** One move in the receiving side's front-door sequence. */
export interface ReceiverStep {
  step: number;
  title: string;
  detail: string;
}

/** The receiving beat: what gated the 2xx, the answer, and the async tail. */
export interface ReceiverRun {
  /** What the front door answered — 200 for the verified exhibit. */
  answeredStatus: number;
  answeredBody: string;
  /** Steps that run BEFORE the answer: raw read, verify. */
  asyncBefore2xx: ReceiverStep[];
  /** Steps that run AFTER the answer: hand-off, dedupe. */
  asyncAfter2xx: ReceiverStep[];
}

/** The webhook simulator's whole payload. */
export interface WebhookSimulatorDemoData {
  config: SimulatorConfig;
  signing: SigningExhibit;
  deliveries: DeliveryLedger[];
  receiver: ReceiverRun;
}
