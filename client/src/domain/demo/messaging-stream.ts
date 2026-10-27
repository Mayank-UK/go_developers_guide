/**
 * Domain models for the messaging topic's stream demo (demo block id
 * `msg:stream`) — the client-side shape of
 * `GET /api/demo/msg/stream`. Field names match the server's camelCase
 * wire contract 1:1 (the DTO is the seam where wire and domain may
 * diverge later).
 *
 * One payload walks the consumer-group lifecycle against the RUNNING
 * stack Redis: the appended entries with their store-assigned ids, the
 * beats in execution order (group, consume, ack, pending, autoclaim,
 * drained), and the three derived proofs — ordering, the queue
 * property, crash recovery.
 */

/** One stream entry observed by the story: its id and its step. */
export interface MessagingEntry {
  id: string;
  step: string;
}

/** One beat of the story: what was done, the commands, the outcome. */
export interface MessagingStreamBeat {
  beat: string;
  commands: string[];
  outcome: string;
  entries: string[];
}

/** The story's three derived verdicts. */
export interface MessagingStreamProofs {
  ordering: string;
  queueProof: string;
  recovery: string;
  pendingLeft: number;
}

/** The whole stream-story payload. */
export interface MessagingStreamDemoData {
  namespace: string;
  stream: string;
  group: string;
  commands: string[];
  beats: MessagingStreamBeat[];
  proofs: MessagingStreamProofs;
  reasoning: string;
}
