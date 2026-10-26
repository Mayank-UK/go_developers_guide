import type {
  AttemptRuling,
  AttemptRow,
  DeadLetter,
  DeliveryLedger,
  DeliveryOutcome,
  IssuedSignature,
  ReceiverRun,
  ReceiverStep,
  SignatureCase,
  SigningExhibit,
  SimulatorConfig,
  WebhookSimulatorDemoData,
} from '../../domain/demo/webhook-simulator';
import {
  isRecord,
  optionalRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * WebhookSimulatorDTO — the wire/transport shape of the webhook
 * delivery-simulator payload. Mirrors the domain model 1:1 (the interface
 * is the seam where wire and domain may diverge); the `fromJson` builds
 * ONLY the listed fields (whitelist) and throws on any shape violation,
 * which parseDto upgrades to an InvalidResponseException.
 *
 * Vocabulary fields (`algorithm`, `ruling`, `outcome`) validate against
 * the server's closed vocabularies so a drifting builder fails here
 * first. `reason` is deliberately `requiredText` (present-but-empty
 * passes): the VERIFIED case carries `""` because no error happened —
 * empty is DATA there, not absence. `deadLetter` is optionalRecord
 * because the server OMITS the wire key for a delivered ledger entry
 * (optionals omitted, never null).
 */

export type WebhookSimulatorDemoDto = WebhookSimulatorDemoData;

const SIMULATOR_ALGORITHMS: readonly string[] = ['HMAC-SHA256'];
const ATTEMPT_RULINGS: readonly string[] = ['retry-scheduled', 'delivered', 'dead-lettered'];
const DELIVERY_OUTCOMES: readonly string[] = ['delivered', 'dead-lettered'];

/** Deserialize + validate the GET /api/demo/api/webhook-simulator payload. */
export function webhookSimulatorDtoFromJson(body: unknown): WebhookSimulatorDemoDto {
  if (!isRecord(body)) throw new Error('webhook-simulator payload must be an object');
  return {
    config: simulatorConfigFromJson(requiredRecord(body, 'config')),
    signing: signingExhibitFromJson(requiredRecord(body, 'signing')),
    deliveries: requiredArray(body, 'deliveries').map(deliveryLedgerFromJson),
    receiver: receiverRunFromJson(requiredRecord(body, 'receiver')),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toWebhookSimulatorDemo(dto: WebhookSimulatorDemoDto): WebhookSimulatorDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function simulatorConfigFromJson(body: Record<string, unknown>): SimulatorConfig {
  const algorithm = requiredString(body, 'algorithm');
  if (!SIMULATOR_ALGORITHMS.includes(algorithm)) {
    throw new Error(`unknown simulator algorithm "${algorithm}"`);
  }
  return {
    algorithm,
    signatureHeader: requiredString(body, 'signatureHeader'),
    headerFormat: requiredString(body, 'headerFormat'),
    messageFormat: requiredString(body, 'messageFormat'),
    toleranceSeconds: requiredNumber(body, 'toleranceSeconds'),
    maxAttempts: requiredNumber(body, 'maxAttempts'),
    baseDelaySeconds: requiredNumber(body, 'baseDelaySeconds'),
    maxDelaySeconds: requiredNumber(body, 'maxDelaySeconds'),
    delayPattern: requiredString(body, 'delayPattern'),
  };
}

function signingExhibitFromJson(body: Record<string, unknown>): SigningExhibit {
  return {
    secretHint: requiredString(body, 'secretHint'),
    body: requiredString(body, 'body'),
    issued: issuedSignatureFromJson(requiredRecord(body, 'issued')),
    cases: requiredArray(body, 'cases').map(signatureCaseFromJson),
  };
}

function issuedSignatureFromJson(body: Record<string, unknown>): IssuedSignature {
  return {
    timestamp: requiredString(body, 'timestamp'),
    nonce: requiredString(body, 'nonce'),
    signature: requiredString(body, 'signature'),
    headerLine: requiredString(body, 'headerLine'),
  };
}

function signatureCaseFromJson(body: unknown): SignatureCase {
  if (!isRecord(body)) throw new Error('signature case must be an object');
  return {
    id: requiredString(body, 'id'),
    description: requiredString(body, 'description'),
    receiverClock: requiredString(body, 'receiverClock'),
    presented: requiredString(body, 'presented'),
    verified: requiredBoolean(body, 'verified'),
    reason: requiredText(body, 'reason'),
    note: requiredString(body, 'note'),
  };
}

function deliveryLedgerFromJson(body: unknown): DeliveryLedger {
  if (!isRecord(body)) throw new Error('delivery ledger must be an object');
  const outcome = requiredString(body, 'outcome');
  if (!DELIVERY_OUTCOMES.includes(outcome)) {
    throw new Error(`unknown delivery outcome "${outcome}"`);
  }
  const rawDeadLetter = optionalRecord(body, 'deadLetter');
  return {
    deliveryId: requiredString(body, 'deliveryId'),
    event: requiredString(body, 'event'),
    endpoint: requiredString(body, 'endpoint'),
    attempts: requiredArray(body, 'attempts').map(attemptRowFromJson),
    outcome: outcome as DeliveryOutcome,
    attemptsUsed: requiredNumber(body, 'attemptsUsed'),
    totalWaitSeconds: requiredNumber(body, 'totalWaitSeconds'),
    deadLetter: rawDeadLetter === undefined ? undefined : deadLetterFromJson(rawDeadLetter),
  };
}

function attemptRowFromJson(body: unknown): AttemptRow {
  if (!isRecord(body)) throw new Error('attempt row must be an object');
  const ruling = requiredString(body, 'ruling');
  if (!ATTEMPT_RULINGS.includes(ruling)) {
    throw new Error(`unknown attempt ruling "${ruling}"`);
  }
  return {
    attempt: requiredNumber(body, 'attempt'),
    waitSeconds: requiredNumber(body, 'waitSeconds'),
    status: requiredNumber(body, 'status'),
    ruling: ruling as AttemptRuling,
  };
}

function deadLetterFromJson(body: Record<string, unknown>): DeadLetter {
  return {
    queue: requiredString(body, 'queue'),
    reason: requiredString(body, 'reason'),
    replayable: requiredBoolean(body, 'replayable'),
  };
}

function receiverRunFromJson(body: Record<string, unknown>): ReceiverRun {
  return {
    answeredStatus: requiredNumber(body, 'answeredStatus'),
    answeredBody: requiredString(body, 'answeredBody'),
    asyncBefore2xx: requiredArray(body, 'asyncBefore2xx').map(receiverStepFromJson),
    asyncAfter2xx: requiredArray(body, 'asyncAfter2xx').map(receiverStepFromJson),
  };
}

function receiverStepFromJson(body: unknown): ReceiverStep {
  if (!isRecord(body)) throw new Error('receiver step must be an object');
  return {
    step: requiredNumber(body, 'step'),
    title: requiredString(body, 'title'),
    detail: requiredString(body, 'detail'),
  };
}
