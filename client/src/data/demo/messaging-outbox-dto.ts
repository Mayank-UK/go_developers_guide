import type {
  MessagingOutboxBeat,
  MessagingOutboxDemoData,
  MessagingOutboxProofs,
} from '../../domain/demo/messaging-outbox';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
} from '../content/validate';

/**
 * MessagingOutboxDTO — the wire/transport shape of the outbox-story
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException. Message ids and
 * stream entry ids are live randomness per run — validated as non-empty
 * strings, never pinned to literals.
 */

export type MessagingOutboxDemoDto = MessagingOutboxDemoData;

function stringArray(source: Record<string, unknown>, key: string): string[] {
  return requiredArray(source, key).map((value) => {
    if (typeof value !== 'string' || value === '') {
      throw new Error(`expected a non-empty string at "${key}"`);
    }
    return value;
  });
}

function beatFromJson(value: unknown): MessagingOutboxBeat {
  if (!isRecord(value)) throw new Error('outbox beat must be an object');
  return {
    beat: requiredString(value, 'beat'),
    commands: stringArray(value, 'commands'),
    outcome: requiredString(value, 'outcome'),
    ids: stringArray(value, 'ids'),
  };
}

function proofsFromJson(body: Record<string, unknown>): MessagingOutboxProofs {
  const proofs = requiredRecord(body, 'proofs');
  return {
    atomicity: requiredString(proofs, 'atomicity'),
    relayProof: requiredString(proofs, 'relayProof'),
    idempotency: requiredString(proofs, 'idempotency'),
    duplicates: requiredNumber(proofs, 'duplicates'),
    eventsDeleted: requiredNumber(proofs, 'eventsDeleted'),
    outboxDeleted: requiredNumber(proofs, 'outboxDeleted'),
  };
}

/** Deserialize + validate the GET /api/demo/msg/outbox payload. */
export function messagingOutboxDtoFromJson(body: unknown): MessagingOutboxDemoDto {
  if (!isRecord(body)) throw new Error('messaging outbox payload must be an object');
  return {
    namespace: requiredString(body, 'namespace'),
    stream: requiredString(body, 'stream'),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    proofs: proofsFromJson(body),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toMessagingOutboxDemo(dto: MessagingOutboxDemoDto): MessagingOutboxDemoData {
  return dto;
}
