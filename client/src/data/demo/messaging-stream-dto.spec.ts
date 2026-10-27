import { describe, expect, it } from 'vitest';
import { messagingStreamDtoFromJson, toMessagingStreamDemo } from './messaging-stream-dto';
import type { MessagingStreamDemoData } from '../../domain/demo/messaging-stream';

/**
 * MessagingStreamDTO spec — hostile-input discipline: only the exact
 * wire shape parses, anything else throws (and therefore surfaces
 * upstream as an invalid_response Failure, never as a guessed default).
 * Entry ids are live randomness per run — asserted by shape only, never
 * pinned to literals.
 */

const wellFormed: MessagingStreamDemoData = {
  namespace: 'gdg:demo:msg:0123456789ab',
  stream: 'gdg:demo:msg:0123456789ab:orders',
  group: 'workers',
  commands: ['XADD', 'XGROUP CREATE', 'XREADGROUP', 'XACK', 'XPENDING', 'XAUTOCLAIM'],
  beats: [
    {
      beat: 'append',
      commands: [
        'XADD gdg:demo:msg:0123456789ab:orders … step created',
        'XADD gdg:demo:msg:0123456789ab:orders … step paid',
      ],
      outcome: 'three entries appended; ids assigned by the store.',
      entries: ['1780000000000-0', '1780000000001-0', '1780000000002-0'],
    },
    {
      beat: 'drained',
      commands: ['XPENDING gdg:demo:msg:0123456789ab:orders workers - + 100'],
      outcome: 'the pending ledger holds 0 entries.',
      entries: [],
    },
  ],
  proofs: {
    ordering: 'consumer-a answered the same ids in the same order — a stream is a log.',
    queueProof: 'consumer-b answered 0 entries: the group had assigned every entry.',
    recovery: 'XAUTOCLAIM moved exactly the stalled entry, values intact.',
    pendingLeft: 0,
  },
  reasoning: 'crash recovery is a command, not a mystery.',
};

describe('messagingStreamDtoFromJson', () => {
  it('parses the exact wire shape', () => {
    expect(messagingStreamDtoFromJson(structuredClone(wellFormed))).toEqual(wellFormed);
  });

  it('rejects non-objects', () => {
    expect(() => messagingStreamDtoFromJson(null)).toThrow();
    expect(() => messagingStreamDtoFromJson('msg')).toThrow();
    expect(() => messagingStreamDtoFromJson([])).toThrow();
  });

  it('rejects missing or malformed beats', () => {
    const missingBeats: unknown = { ...wellFormed, beats: undefined };
    expect(() => messagingStreamDtoFromJson(missingBeats)).toThrow();

    const emptyOutcome: unknown = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], outcome: '' }],
    };
    expect(() => messagingStreamDtoFromJson(emptyOutcome)).toThrow();

    const emptyEntry: unknown = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], entries: ['', '1780000000001-0'] }],
    };
    expect(() => messagingStreamDtoFromJson(emptyEntry)).toThrow();
  });

  it('rejects missing or malformed proofs', () => {
    const noProofs: unknown = { ...wellFormed, proofs: undefined };
    expect(() => messagingStreamDtoFromJson(noProofs)).toThrow();

    const stringPending: unknown = {
      ...wellFormed,
      proofs: { ...wellFormed.proofs, pendingLeft: '0' },
    };
    expect(() => messagingStreamDtoFromJson(stringPending)).toThrow();
  });

  it('converts to the domain model identically while 1:1', () => {
    const dto = messagingStreamDtoFromJson(structuredClone(wellFormed));
    expect(toMessagingStreamDemo(dto)).toEqual(wellFormed);
  });
});
