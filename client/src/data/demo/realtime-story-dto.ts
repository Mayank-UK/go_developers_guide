import type {
  RealtimeStoryBeat,
  RealtimeStoryDemoData,
  RealtimeStoryFact,
} from '../../domain/demo/realtime-story';
import { isRecord, requiredArray, requiredString } from '../content/validate';

/**
 * RealtimeStoryDTO — the wire/transport shape of the realtime demos'
 * shared story payload. Mirrors the domain model 1:1; the `fromJson`
 * builds ONLY the listed fields (whitelist) and throws on any shape
 * violation, which parseDto upgrades to an InvalidResponseException.
 * Verdicts validate against the server's closed vocabulary
 * (ok/rejected/info), and story prose — including every derived frame
 * byte and hub count — validates as non-empty strings, never pinned to
 * literals (the server derives them per run).
 */

export type RealtimeStoryDemoDto = RealtimeStoryDemoData;

const VERDICTS = new Set(['ok', 'rejected', 'info']);

function factFromJson(value: unknown): RealtimeStoryFact {
  if (!isRecord(value)) throw new Error('realtime story fact must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredString(value, 'value'),
  };
}

function beatFromJson(value: unknown): RealtimeStoryBeat {
  if (!isRecord(value)) throw new Error('realtime story beat must be an object');
  const verdict = requiredString(value, 'verdict');
  if (!VERDICTS.has(verdict)) {
    throw new Error(`unknown realtime story verdict "${verdict}" (expected ok|rejected|info)`);
  }
  return {
    beat: requiredString(value, 'beat'),
    detail: requiredString(value, 'detail'),
    verdict,
    facts: requiredArray(value, 'facts').map(factFromJson),
  };
}

/** Deserialize + validate one realtime story payload (either demo). */
export function realtimeStoryDtoFromJson(body: unknown): RealtimeStoryDemoDto {
  if (!isRecord(body)) throw new Error('realtime story payload must be an object');
  return {
    mechanism: requiredString(body, 'mechanism'),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    proofs: requiredArray(body, 'proofs').map(factFromJson),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toRealtimeStoryDemo(dto: RealtimeStoryDemoDto): RealtimeStoryDemoData {
  return dto;
}

/** The story endpoints' paths, keyed by the demo registry's ids. */
export const REALTIME_STORY_PATHS = {
  ticker: '/demo/realtime/ticker',
  chat: '/demo/realtime/chat',
} as const;
