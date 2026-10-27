/**
 * Domain models for the i18n topic's `i18n:negotiator` demo story —
 * the same story discipline the files and security topics established:
 * a mechanism name, beats in execution order (each with a teaching
 * sentence, a verdict color, and label/value facts), derived proofs,
 * and a closing reasoning paragraph. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and
 * domain may diverge later).
 */

/** One label/value pair — q-values, chain steps, per-locale renderings. */
export interface I18nStoryFact {
  label: string;
  value: string;
}

/** One beat of the negotiator story: what was done, the sentence, the color. */
export interface I18nStoryBeat {
  beat: string;
  detail: string;
  /** 'ok' | 'rejected' | 'info' — the client colors, never interprets. */
  verdict: string;
  facts: I18nStoryFact[];
}

/** The whole story payload the negotiator endpoint serves. */
export interface I18nStoryDemoData {
  mechanism: string;
  beats: I18nStoryBeat[];
  proofs: I18nStoryFact[];
  reasoning: string;
}
