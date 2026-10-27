import { describe, expect, it } from 'vitest';
import { i18nStoryDtoFromJson, toI18nStoryDemo } from './i18n-story-dto';
import type { I18nStoryDemoData } from '../../domain/demo/i18n-story';

/**
 * I18nStoryDTO spec — whitelist parsing with shape violations throwing
 * (parseDto upgrades them to InvalidResponseException at the boundary).
 * Verdicts validate against the server's closed ok/rejected/info
 * vocabulary; story prose validates as non-empty strings, never pinned
 * to literals.
 */

const wellFormed: I18nStoryDemoData = {
  mechanism: 'i18n-negotiator',
  beats: [
    {
      beat: 'the header, parsed',
      detail:
        'the story runs against THIS request\u2019s own header: "de-AT, en;q=0.8". The parser split it into 2 usable candidate(s).',
      verdict: 'info',
      facts: [
        { label: 'de-at', value: 'q=1.000' },
        { label: 'en', value: 'q=0.800' },
      ],
    },
    {
      beat: 'the match',
      detail: 'the negotiator walked the candidates most-specific first and selected the "de" catalog via base.',
      verdict: 'ok',
      facts: [{ label: 'de-at (q=1.0)', value: 'base \u2192 de' }],
    },
  ],
  proofs: [
    {
      label: 'the chain',
      value: 'exact \u2192 base \u2192 wildcard \u2192 default; this request selected de via base',
    },
  ],
  reasoning:
    'a zone changes the clock reading, a locale changes everything else; the default catalog is a never-fail last link.',
};

describe('i18nStoryDtoFromJson', () => {
  it('accepts a well-formed payload unchanged', () => {
    expect(i18nStoryDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('rejects an unknown verdict', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], verdict: 'maybe' }],
    };

    expect(() => i18nStoryDtoFromJson(poisoned)).toThrow(/unknown i18n story verdict/);
  });

  it('rejects a beat whose facts carry a non-object', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], facts: [{ label: 'x', value: 'y' }, 'not a fact'] }],
    };

    expect(() => i18nStoryDtoFromJson(poisoned)).toThrow(/i18n story fact must be an object/);
  });

  it('rejects a payload that is not an object', () => {
    expect(() => i18nStoryDtoFromJson('nope')).toThrow(/i18n story payload must be an object/);
  });

  it('rejects a payload missing its reasoning', () => {
    const { reasoning: _reasoning, ...withoutReasoning } = wellFormed;

    expect(() => i18nStoryDtoFromJson(withoutReasoning)).toThrow(/reasoning/);
  });

  it('converts to the domain model identically (1:1 today)', () => {
    expect(toI18nStoryDemo(i18nStoryDtoFromJson(wellFormed))).toEqual(wellFormed);
  });
});
