/**
 * Content domain models — the pure shapes of the guide's teaching content:
 * index (groups + topic summaries) and one topic's detail (sections → blocks).
 * They know nothing about JSON or HTTP; the data layer validates wire payloads
 * into these shapes (see data/content/content-dto.ts).
 *
 * Const-object + literal-union style throughout (no enums — erasable syntax):
 * `ContentBlockKind` exists so code that needs the kind *values* as data (the
 * DTO parser) can loop/compare without importing string literals ad hoc.
 */

/** The block kinds the content API serves (as-const, not an enum). */
export const ContentBlockKind = {
  Text: 'text',
  Code: 'code',
  Demo: 'demo',
  List: 'list',
} as const;

export type ContentBlockKind = (typeof ContentBlockKind)[keyof typeof ContentBlockKind];

/* ---- content index (GET /api/content) ---- */

export interface TopicGroup {
  id: string;
  title: string;
  order: number;
}

export interface TopicSummary {
  id: string;
  title: string;
  subtitle?: string;
  group: string;
  order: number;
  sectionCount: number;
}

export interface ContentIndex {
  version: number;
  groups: TopicGroup[];
  topics: TopicSummary[];
}

/* ---- topic detail (GET /api/content/{topicId}) ---- */

export interface TextBlockData {
  kind: 'text';
  id: string;
  /** Prose; `\n\n` separates paragraphs (the renderer splits on it). */
  text: string;
}

export interface CodeBlockData {
  kind: 'code';
  id: string;
  language: string;
  caption?: string;
  /** Repo-root-relative path of the real source this snippet is quoted from. */
  source?: string;
  /** The displayed code itself. */
  snippet: string;
}

export interface DemoBlockData {
  kind: 'demo';
  /**
   * Doubles as the demo id (`<area>:<name>`, e.g. `runtime:async`) — the
   * client demo registry's lookup key.
   */
  id: string;
  title: string;
  description?: string;
}

export interface ListBlockData {
  kind: 'list';
  id: string;
  /** True only when sequence matters — the renderer maps it to `<ol>` vs `<ul>`. */
  ordered: boolean;
  /**
   * 3–8 flat items of "Lead term — explanation." grammar (block-selection
   * standard); validated at the DTO boundary, never re-checked downstream.
   */
  items: string[];
}

/** Discriminated union on `kind` — the topic renderer switches exhaustively. */
export type ContentBlock = TextBlockData | CodeBlockData | DemoBlockData | ListBlockData;

export interface TopicSection {
  id: string;
  title: string;
  lead?: string;
  blocks: ContentBlock[];
}

export interface TopicDetail {
  version: number;
  id: string;
  title: string;
  subtitle?: string;
  group: string;
  order: number;
  sections: TopicSection[];
}
