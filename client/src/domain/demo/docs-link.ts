/**
 * Domain models for the api-documentation topic's docs-link demo — the
 * client-side shape of `GET /api/demo/api/docs-link`. Field names match
 * the server's camelCase wire contract 1:1 (the DTO is the seam where
 * wire and domain may diverge later).
 */

/** The spec's own header facts, parsed by the server from the real file. */
export interface DocsLinkSpecMeta {
  /** What info.title says, e.g. "Go Developers Guide — posts API". */
  title: string;
  /** The API version info.version carries ("1.0.0"). */
  version: string;
  /** The OpenAPI version the document targets ("3.1.0"). */
  openapiVersion: string;
  /** Repo-root-relative location of the spec file. */
  specPath: string;
  /** The route that serves the spec verbatim (/api/openapi.yaml). */
  specURL: string;
  /** Base URLs the document names, verbatim. */
  servers: string[];
  /** Path items under `paths`. */
  pathCount: number;
  /** Operations across all path items. */
  operationCount: number;
  /** Named schemas under components/schemas. */
  schemaCount: number;
  /** One-line note on what keeps the facts honest. */
  validationIssue: string;
}

/** One row of the spec's operation inventory. */
export interface DocsLinkOperation {
  /** The spec's unique operationId (e.g. listPosts). */
  id: string;
  /** Uppercase HTTP verb. */
  method: string;
  /** The path template the operation lives at. */
  path: string;
  /** The spec's one-line summary. */
  summary: string;
}

/** One named schema the spec defines. */
export interface DocsLinkSchema {
  name: string;
  description: string;
}

/** The docs-serving story: where the contract lives at runtime. */
export interface DocsLinkServing {
  specURL: string;
  contentType: string;
  servedVerbatim: boolean;
  swaggerStory: string;
}

/** One spec-vs-wire guarantee the drift test enforces. */
export interface DocsLinkDriftRow {
  check: string;
  compares: string;
  settled: string;
}

export interface DocsLinkDemoData {
  spec: DocsLinkSpecMeta;
  operations: DocsLinkOperation[];
  schemas: DocsLinkSchema[];
  serving: DocsLinkServing;
  drift: DocsLinkDriftRow[];
  /** The verbatim spec text — the very bytes /api/openapi.yaml serves. */
  specText: string;
}
