import type { I18nStoryDemoData } from '../../../domain/demo/i18n-story';

/**
 * The shared fixture for the negotiator demo spec — shaped exactly like
 * the server's payload, with the topic's own distinctive beats so the
 * spec pins the demo's teaching content (the parsed candidates with
 * their q-values, the fallback chain's selection, the per-locale
 * greetings, the plural boundary at 0, the four format rows, the
 * timezone beat with its honest unknown-zone error) rather than generic
 * story chrome.
 */

/** A negotiator-shaped story: the seven beats, honest to the wire shape. */
export function i18nNegotiatorFixture(
  overrides: Partial<I18nStoryDemoData> = {},
): I18nStoryDemoData {
  return {
    mechanism: 'i18n-negotiator',
    beats: [
      {
        beat: 'the header, parsed',
        detail:
          'the story runs against THIS request\u2019s own header: "fr-CH, fr;q=0.9, en;q=0.8". The parser split it into 3 usable candidate(s).',
        verdict: 'info',
        facts: [
          { label: 'fr-ch', value: 'q=1.000' },
          { label: 'fr', value: 'q=0.900' },
          { label: 'en', value: 'q=0.800' },
        ],
      },
      {
        beat: 'the match',
        detail:
          'the negotiator walked the candidates most-specific first and selected the "fr" catalog via base.',
        verdict: 'ok',
        facts: [{ label: 'fr-ch (q=1.0)', value: 'base \u2192 fr' }],
      },
      {
        beat: 'one message, every catalog',
        detail: 'the SAME key (demo.greeting, {name} interpolated with "Ada") rendered by each carried catalog.',
        verdict: 'ok',
        facts: [
          { label: 'en', value: 'Welcome back, Ada!' },
          { label: 'de', value: 'Willkommen zur\u00fcck, Ada!' },
          { label: 'fr', value: 'Bienvenue, Ada !' },
          { label: 'ja', value: '\u304a\u304b\u3048\u308a\u306a\u3055\u3044\u3001Ada\u3055\u3093\uff01' },
        ],
      },
      {
        beat: 'plurals across locales',
        detail:
          'the singular counts each locale\u2019s rule selects: en: 1 \u00b7 de: 1 \u00b7 fr: 0 and 1 \u00b7 ja: no singular at all. Same counts, three rules, one engine.',
        verdict: 'ok',
        facts: [{ label: 'fr \u00b7 0 (one)', value: 'Tu as 0 nouvelle publication dans ton fil.' }],
      },
      {
        beat: 'locale formats',
        detail: 'the number 1234567.89, an 1899-minor-unit price, the date of the story instant, and its clock \u2014 per locale.',
        verdict: 'ok',
        facts: [
          { label: 'en', value: '1,234,567.89 \u00b7 $18.99 \u00b7 Oct 27, 2026 \u00b7 12:30 PM' },
          { label: 'de', value: '1.234.567,89 \u00b7 18,99 \u20ac \u00b7 27.10.2026 \u00b7 12:30' },
          { label: 'fr', value: '1\u202f234\u202f567,89 \u00b7 18,99 \u20ac \u00b7 27 octobre 2026 \u00b7 12:30' },
          { label: 'ja', value: '1,234,567.89 \u00b7 \u00a51,899 \u00b7 2026\u5e7410\u670827\u65e5 \u00b7 12:30' },
        ],
      },
      {
        beat: 'timezone is not locale',
        detail:
          'the SAME instant (2026-10-27T12:30:00Z) read in three IANA zones. Unknown zones answer an error, verbatim: "unknown time zone Mars/Olympus".',
        verdict: 'info',
        facts: [
          { label: 'Europe/Berlin (CET, UTC+01:00)', value: 'de clock 13:30 \u00b7 en clock 1:30 PM' },
          { label: 'America/New_York (EDT, UTC-04:00)', value: 'de clock 08:30 \u00b7 en clock 8:30 AM' },
        ],
      },
      {
        beat: 'the honest edges',
        detail:
          'an unsupported language ("ko-KR,ko;q=0.9" \u2014 no Korean catalog) walked its chain to the default. A missing key was served as the key itself: never a 500.',
        verdict: 'ok',
        facts: [
          { label: 'fallback chain', value: 'ko-kr\u2192none ko\u2192none' },
          { label: 'missing-key log', value: 'missing i18n catalog key "demo.never_translated" for "de"' },
        ],
      },
    ],
    proofs: [
      {
        label: 'the chain',
        value: 'exact \u2192 base \u2192 wildcard \u2192 default; this request selected fr via base',
      },
      {
        label: 'separators at a glance',
        value: 'en 1,234,567.89 \u00b7 de 1.234.567,89 \u00b7 fr 1\u202f234\u202f567,89 \u00b7 ja 1,234,567.89 \u2014 one value',
      },
      {
        label: 'zone offsets',
        value: 'Europe/Berlin UTC+01:00 \u00b7 America/New_York UTC-04:00 \u00b7 Asia/Tokyo UTC+09:00 \u2014 one instant',
      },
    ],
    reasoning:
      'The negotiator turns the header into one supported tag; the catalog turns keys into text; the formatters render per locale while time.LoadLocation keeps TIME a separate axis.',
    ...overrides,
  };
}
