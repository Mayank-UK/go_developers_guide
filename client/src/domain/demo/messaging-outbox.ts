/**
 * Domain models for the messaging topic's outbox demo (demo block id
 * `msg:outbox`) — the client-side shape of
 * `GET /api/demo/msg/outbox`. Field names match the server's camelCase
 * wire contract 1:1.
 *
 * One payload walks the transactional outbox across BOTH live stores:
 * the beats in execution order (committed record pairs, the deliberate
 * rollback, the relay pass and its empty second run, the idempotent
 * consume, the refused redelivery, the cleanup) and the derived proofs
 * — atomicity, relay idempotency, consumer idempotency.
 */

/** One beat of the story: what was done, the commands, the outcome. */
export interface MessagingOutboxBeat {
  beat: string;
  commands: string[];
  outcome: string;
  ids: string[];
}

/** The story's three derived verdicts plus the cleanup counts. */
export interface MessagingOutboxProofs {
  atomicity: string;
  relayProof: string;
  idempotency: string;
  duplicates: number;
  eventsDeleted: number;
  outboxDeleted: number;
}

/** The whole outbox-story payload. */
export interface MessagingOutboxDemoData {
  namespace: string;
  stream: string;
  beats: MessagingOutboxBeat[];
  proofs: MessagingOutboxProofs;
  reasoning: string;
}
