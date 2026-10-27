import type {
  I18nStoryBeat,
  I18nStoryDemoData,
  I18nStoryFact,
} from '../../domain/demo/i18n-story';
import { isRecord, requiredArray, requiredString } from '../content/validate';

/**
 * I18nStoryDTO — the wire/transport shape of the i18n topic's
 * negotiator story payload. Mirrors the domain model 1:1; the
 * `fromJson` builds ONLY the listed fields (whitelist) and throws on
 * any shape violation, which parseDto upgrades to an
 * InvalidResponseException. Verdicts validate against the server's
 * closed vocabulary (ok/rejected/info), and story prose is
 * server-minted per run — validated as non-empty strings, never pinned
 * to literals.
 */

export type I18nStoryDemoDto = I18nStoryDemoData;

const VERDICTS = new Set(['ok', 'rejected', 'info']);

function factFromJson(value: unknown): I18nStoryFact {
  if (!isRecord(value)) throw new Error('i18n story fact must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredString(value, 'value'),
  };
}

function beatFromJson(value: unknown): I18nStoryBeat {
  if (!isRecord(value)) throw new Error('i18n story beat must be an object');
  const verdict = requiredString(value, 'verdict');
  if (!VERDICTS.has(verdict)) {
    throw new Error(`unknown i18n story verdict "${verdict}" (expected ok|rejected|info)`);
  }
  return {
    beat: requiredString(value, 'beat'),
    detail: requiredString(value, 'detail'),
    verdict,
    facts: requiredArray(value, 'facts').map(factFromJson),
  };
}

/** Deserialize + validate one negotiator story payload. */
export function i18nStoryDtoFromJson(body: unknown): I18nStoryDemoDto {
  if (!isRecord(body)) throw new Error('i18n story payload must be an object');
  return {
    mechanism: requiredString(body, 'mechanism'),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    proofs: requiredArray(body, 'proofs').map(factFromJson),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toI18nStoryDemo(dto: I18nStoryDemoDto): I18nStoryDemoData {
  return dto;
}

/** The negotiator story endpoint's path (the demo registry's id is `i18n:negotiator`). */
export const I18N_STORY_PATH = '/demo/i18n/negotiator' as const;
