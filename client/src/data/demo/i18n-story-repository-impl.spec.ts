import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { I18nStoryApi } from './i18n-story-api';
import { I18nStoryRepositoryImpl } from './i18n-story-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * I18nStoryRepositoryImpl spec — fixture-driven (happy + failure): the
 * negotiator story payload (seven beats with verdicts and facts,
 * proofs, reasoning) round-trips; a poisoned payload and a network
 * failure resolve as Failed Results, never throws.
 */

const storyPayload = {
  mechanism: 'i18n-negotiator',
  beats: [
    {
      beat: 'the header, parsed',
      detail: 'the parser split "fr-CH, fr;q=0.9, en;q=0.8" into 3 usable candidate(s).',
      verdict: 'info',
      facts: [
        { label: 'fr-ch', value: 'q=1.000' },
        { label: 'fr', value: 'q=0.900' },
        { label: 'en', value: 'q=0.800' },
      ],
    },
    {
      beat: 'the match',
      detail: 'selected the "fr" catalog via base.',
      verdict: 'ok',
      facts: [{ label: 'fr-ch (q=1.0)', value: 'base \u2192 fr' }],
    },
    {
      beat: 'one message, every catalog',
      detail: 'the SAME key rendered by each carried catalog.',
      verdict: 'ok',
      facts: [{ label: 'de', value: 'Willkommen zur\u00fcck, Ada!' }],
    },
    {
      beat: 'plurals across locales',
      detail: 'French singularizes 0 AND 1; Japanese never distinguishes.',
      verdict: 'ok',
      facts: [{ label: 'fr \u00b7 0 (one)', value: 'Tu as 0 nouvelle publication dans ton fil.' }],
    },
    {
      beat: 'locale formats',
      detail: 'grouping separator, decimal separator, currency symbol, month names, clock.',
      verdict: 'ok',
      facts: [{ label: 'de', value: '1.234.567,89 \u00b7 18,99 \u20ac \u00b7 27.10.2026 \u00b7 12:30' }],
    },
    {
      beat: 'timezone is not locale',
      detail: 'the SAME instant read in three IANA zones.',
      verdict: 'info',
      facts: [{ label: 'Europe/Berlin (CET, UTC+01:00)', value: 'de clock 13:30 \u00b7 en clock 1:30 PM' }],
    },
    {
      beat: 'the honest edges',
      detail: 'an unsupported language walked its chain to the default; a missing key served as itself.',
      verdict: 'ok',
      facts: [{ label: 'fallback chain', value: 'ko-kr\u2192none ko\u2192none' }],
    },
  ],
  proofs: [
    { label: 'separators at a glance', value: 'en 1,234,567.89 \u00b7 de 1.234.567,89 \u00b7 fr 1\u202f234\u202f567,89 \u00b7 ja 1,234,567.89' },
  ],
  reasoning: 'locale picks the words and the shape, the zone picks the reading.',
};

function makeRepository(): I18nStoryRepositoryImpl {
  return new I18nStoryRepositoryImpl(new I18nStoryApi(createAppConfig(), '/demo/i18n/negotiator'));
}

describe('I18nStoryRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/i18n/negotiator', body: storyPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.mechanism).toBe('i18n-negotiator');
      expect(result.value.beats).toHaveLength(7);
      expect(result.value.beats[0]?.verdict).toBe('info');
      expect(result.value.beats[1]?.facts[0]?.value).toBe('base \u2192 fr');
      expect(result.value.proofs[0]?.label).toBe('separators at a glance');
      expect(result.value.reasoning).toContain('zone picks the reading');
    }
  });

  it('rejects a payload whose beat verdict is unknown', async () => {
    const poisoned = {
      ...storyPayload,
      beats: [{ ...storyPayload.beats[0], verdict: 'maybe' }],
    };
    stubApiFetch([{ url: '/api/demo/i18n/negotiator', body: poisoned }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
