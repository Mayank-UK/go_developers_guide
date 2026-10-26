/**
 * Domain models for the validation topic's arena demo — the client-side
 * shape of `GET /api/demo/validation/arena`. Field names match the
 * server's camelCase wire contract 1:1 (the DTO is the seam where wire
 * and domain may diverge later).
 */

/** The three layers the topic teaches, in stack order. */
export type ArenaLayerId = 'decode' | 'rules' | 'library';

/** Layer one's verdict on a candidate body. */
export type ArenaDecodeStatus = 'accepted' | 'rejected';

/** Layer two's verdict — skipped when the decode layer already refused. */
export type ArenaRulesStatus = 'validated' | 'rejected' | 'skipped';

/** One validation failure: which wire field broke which rule, and what it carried. */
export interface ArenaIssue {
  field: string;
  rule: string;
  got: string;
}

/** One field of the judged form: wire name, Go type, rule, required-ness. */
export interface ArenaFieldSpec {
  name: string;
  type: string;
  rule: string;
  required: boolean;
}

/** The request struct the arena judges, with every field's rule stated once. */
export interface ArenaSubject {
  name: string;
  source: string;
  fields: ArenaFieldSpec[];
}

/** One of the three layers, named so the stack can render in order. */
export interface ArenaLayerCard {
  id: ArenaLayerId;
  title: string;
  tool: string;
  summary: string;
}

/** Layer one's verdict, with the decoder's own error when it refused. */
export interface ArenaDecodeOutcome {
  status: ArenaDecodeStatus;
  errorType?: string;
  error?: string;
}

/** Layer two's verdict, with the collected issues when it rejected. */
export interface ArenaRulesOutcome {
  status: ArenaRulesStatus;
  issues?: ArenaIssue[];
}

/** The decoded, validated form an accepted candidate echoes back. */
export interface ArenaValidatedForm {
  title: string;
  body: string;
  score: number;
  email: string;
}

/**
 * The composed HTTP outcome: 200 with the validated form, or 422 with the
 * REAL error envelope (produced by the server's central error writer).
 * Both optionals are omitted on the wire when absent, never null.
 */
export interface ArenaResponse {
  status: number;
  envelope?: Record<string, unknown>;
  validated?: ArenaValidatedForm;
}

/** One exhibit input fully judged: the raw body, both verdicts, the response. */
export interface ArenaCandidate {
  id: string;
  label: string;
  expects: string;
  body: string;
  decode: ArenaDecodeOutcome;
  validate: ArenaRulesOutcome;
  response: ArenaResponse;
  note?: string;
}

/** One field of the tag-ecosystem comparison: tag data vs the hand-rolled rule. */
export interface ArenaTagRow {
  field: string;
  tag: string;
  libraryRule: string;
  handRolled: string;
}

/** The library-comparison face — honest that no validator package is imported. */
export interface ArenaComparison {
  library: string;
  imported: boolean;
  rows: ArenaTagRow[];
  note: string;
}

/** One piece of the 422 envelope and why it is shaped that way. */
export interface ArenaAnatomyRow {
  path: string;
  value: string;
  why: string;
}

/** The dissected 422: the real envelope body for the multi-error candidate. */
export interface ArenaEnvelopeShow {
  candidateId: string;
  status: number;
  body: Record<string, unknown>;
  anatomy: ArenaAnatomyRow[];
}

/** Wire shape of GET /api/demo/validation/arena (camelCase throughout). */
export interface ValidationArenaDemoData {
  subject: ArenaSubject;
  layers: ArenaLayerCard[];
  candidates: ArenaCandidate[];
  comparison: ArenaComparison;
  envelope: ArenaEnvelopeShow;
  reasoning: string;
}
