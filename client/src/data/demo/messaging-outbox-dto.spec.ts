import { describe, expect, it } from 'vitest';
import { messagingOutboxDtoFromJson, toMessagingOutboxDemo } from './messaging-outbox-dto';
import type { MessagingOutboxDemoData } from '../../domain/demo/messaging-outbox';

/**
 * MessagingOutboxDTO spec — hostile-input discipline: only the exact
 * wire shape parses, anything else throws (and therefore surfaces
 * upstream as an invalid_response Failure, never as a guessed default).
 * Message ids are live randomness per run — asserted by shape only,
 * never pinned to literals.
 */

const wellFormed: MessagingOutboxDemoData = {
  namespace: 'gdg:demo:msg:0123456789ab',
  stream: 'gdg:demo:msg:0123456789ab:outbox',
  beats: [
    {
      beat: 'record (commit)',
      commands: ['BEGIN', 'INSERT INTO msgdemo_events …', 'INSERT INTO msgdemo_outbox …', 'COMMIT'],
      outcome: 'two event rows and their outbox rows committed.',
      ids: ['m-aaaaaaaaaaaaaaaa', 'm-bbbbbbbbbbbbbbbb'],
    },
    {
      beat: 'record (rollback)',
      commands: ['BEGIN', 'INSERT …', 'ROLLBACK'],
      outcome: 'the count query answers 0 — the crash window has no state in it.',
      ids: [],
    },
  ],
  proofs: {
    atomicity: 'the count query answered 0 — all-or-nothing is not a hope.',
    relayProof: 'a second pass drained 0 — published rows never ride twice.',
    idempotency: 'the redelivery was refused 1 time(s).',
    duplicates: 1,
    eventsDeleted: 2,
    outboxDeleted: 2,
  },
  reasoning: 'exactly-once EFFECT is one SET NX away.',
};

describe('messagingOutboxDtoFromJson', () => {
  it('parses the exact wire shape', () => {
    expect(messagingOutboxDtoFromJson(structuredClone(wellFormed))).toEqual(wellFormed);
  });

  it('rejects non-objects', () => {
    expect(() => messagingOutboxDtoFromJson(null)).toThrow();
    expect(() => messagingOutboxDtoFromJson('msg')).toThrow();
    expect(() => messagingOutboxDtoFromJson([])).toThrow();
  });

  it('rejects missing or malformed beats', () => {
    const missingBeats: unknown = { ...wellFormed, beats: undefined };
    expect(() => messagingOutboxDtoFromJson(missingBeats)).toThrow();

    const emptyCommand: unknown = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], commands: ['BEGIN', ''] }],
    };
    expect(() => messagingOutboxDtoFromJson(emptyCommand)).toThrow();

    const emptyId: unknown = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], ids: [''] }],
    };
    expect(() => messagingOutboxDtoFromJson(emptyId)).toThrow();
  });

  it('rejects missing or malformed proofs', () => {
    const noProofs: unknown = { ...wellFormed, proofs: undefined };
    expect(() => messagingOutboxDtoFromJson(noProofs)).toThrow();

    const stringDuplicates: unknown = {
      ...wellFormed,
      proofs: { ...wellFormed.proofs, duplicates: '1' },
    };
    expect(() => messagingOutboxDtoFromJson(stringDuplicates)).toThrow();
  });

  it('converts to the domain model identically while 1:1', () => {
    const dto = messagingOutboxDtoFromJson(structuredClone(wellFormed));
    expect(toMessagingOutboxDemo(dto)).toEqual(wellFormed);
  });
});
