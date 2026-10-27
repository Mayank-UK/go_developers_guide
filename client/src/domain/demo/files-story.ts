/**
 * Domain models for the files topic's `files:workbench` demo story —
 * the same story discipline the auth and security topics established:
 * a mechanism name, beats in execution order (each with a teaching
 * sentence, a verdict color, and label/value facts), derived proofs,
 * and a closing reasoning paragraph. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and
 * domain may diverge later).
 */

/** One label/value pair — byte counts, header values, refusal verdicts. */
export interface FilesStoryFact {
  label: string;
  value: string;
}

/** One beat of the workbench story: what was done, the sentence, the color. */
export interface FilesStoryBeat {
  beat: string;
  detail: string;
  /** 'ok' | 'rejected' | 'info' — the client colors, never interprets. */
  verdict: string;
  facts: FilesStoryFact[];
}

/** The whole story payload the workbench endpoint serves. */
export interface FilesStoryDemoData {
  mechanism: string;
  beats: FilesStoryBeat[];
  proofs: FilesStoryFact[];
  reasoning: string;
}
