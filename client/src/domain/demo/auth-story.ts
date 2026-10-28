/**
 * Domain models for the authentication topic's SEVEN demo stories (demo
 * block ids `auth:sessions`, `auth:api-keys`, `auth:compare`,
 * `auth:jwt`, `auth:oauth`, `auth:hashing`, `auth:refresh`) — one
 * shared shape, because the server side deliberately serves all seven
 * as the same wire structure: a mechanism name, beats in execution
 * order (each with a teaching sentence, a verdict color, and
 * label/value facts), derived proofs, and a closing reasoning
 * paragraph. Field names match the server's camelCase wire contract 1:1
 * (the DTO is the seam where wire and domain may diverge later).
 */

/** One label/value pair — cookie flags, timings, parameters, previews. */
export interface AuthStoryFact {
  label: string;
  value: string;
}

/**
 * The verdict vocabulary — a CLOSED union mirroring the server's four
 * auth-story constants (AuthVerdictOK/Rejected/Alarm/Info in
 * internal/domain/demos/auth_story.go). The client colors, never
 * interprets; the DTO rejects anything outside the set.
 */
export type AuthStoryVerdict = 'ok' | 'rejected' | 'alarm' | 'info';

/** One beat of a story: what was done, the sentence, the verdict color. */
export interface AuthStoryBeat {
  beat: string;
  detail: string;
  /** The verdict color — the client colors, never interprets. */
  verdict: AuthStoryVerdict;
  facts: AuthStoryFact[];
}

/** The whole story payload every auth demo endpoint serves. */
export interface AuthStoryDemoData {
  mechanism: string;
  beats: AuthStoryBeat[];
  proofs: AuthStoryFact[];
  reasoning: string;
}
