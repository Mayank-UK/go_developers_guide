/**
 * Domain models for the security topic's TWO demo stories (demo block
 * ids `security:lab` and `security:pii`) — one shared shape, because
 * the server side deliberately serves both as the same wire structure:
 * a mechanism name, beats in execution order (each with a teaching
 * sentence, a verdict color, and label/value facts), derived proofs,
 * and a closing reasoning paragraph. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and
 * domain may diverge later).
 */

/** One label/value pair — header values, retry hints, classifications. */
export interface SecurityStoryFact {
  label: string;
  value: string;
}

/** One beat of a story: what was done, the sentence, the verdict color. */
export interface SecurityStoryBeat {
  beat: string;
  detail: string;
  /** 'ok' | 'rejected' | 'info' — the client colors, never interprets. */
  verdict: string;
  facts: SecurityStoryFact[];
}

/** The whole story payload both security demo endpoints serve. */
export interface SecurityStoryDemoData {
  mechanism: string;
  beats: SecurityStoryBeat[];
  proofs: SecurityStoryFact[];
  reasoning: string;
}
