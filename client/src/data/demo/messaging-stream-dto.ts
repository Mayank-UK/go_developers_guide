import type {
  MessagingStreamBeat,
  MessagingStreamDemoData,
  MessagingStreamProofs,
} from '../../domain/demo/messaging-stream';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
} from '../content/validate';

/**
 * MessagingStreamDTO — the wire/transport shape of the stream-story
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException. Entry ids are live
 * randomness per run — validated as non-empty strings, never pinned to
 * literals.
 */

export type MessagingStreamDemoDto = MessagingStreamDemoData;

function stringArray(source: Record<string, unknown>, key: string): string[] {
  return requiredArray(source, key).map((value) => {
    if (typeof value !== 'string' || value === '') {
      throw new Error(`expected a non-empty string at "${key}"`);
    }
    return value;
  });
}

function beatFromJson(value: unknown): MessagingStreamBeat {
  if (!isRecord(value)) throw new Error('stream beat must be an object');
  return {
    beat: requiredString(value, 'beat'),
    commands: stringArray(value, 'commands'),
    outcome: requiredString(value, 'outcome'),
    entries: stringArray(value, 'entries'),
  };
}

function proofsFromJson(body: Record<string, unknown>): MessagingStreamProofs {
  const proofs = requiredRecord(body, 'proofs');
  return {
    ordering: requiredString(proofs, 'ordering'),
    queueProof: requiredString(proofs, 'queueProof'),
    recovery: requiredString(proofs, 'recovery'),
    pendingLeft: requiredNumber(proofs, 'pendingLeft'),
  };
}

/** Deserialize + validate the GET /api/demo/msg/stream payload. */
export function messagingStreamDtoFromJson(body: unknown): MessagingStreamDemoDto {
  if (!isRecord(body)) throw new Error('messaging stream payload must be an object');
  return {
    namespace: requiredString(body, 'namespace'),
    stream: requiredString(body, 'stream'),
    group: requiredString(body, 'group'),
    commands: stringArray(body, 'commands'),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    proofs: proofsFromJson(body),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toMessagingStreamDemo(dto: MessagingStreamDemoDto): MessagingStreamDemoData {
  return dto;
}
