import type {
  DocsLinkDemoData,
  DocsLinkDriftRow,
  DocsLinkOperation,
  DocsLinkSchema,
  DocsLinkServing,
  DocsLinkSpecMeta,
} from '../../domain/demo/docs-link';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * DocsLinkDTO — the wire/transport shape of the docs-link payload.
 * Mirrors the domain model 1:1 (the interface is the seam where wire and
 * domain may diverge); the `fromJson` builds ONLY the listed fields
 * (whitelist) and throws on any shape violation, which parseDto upgrades
 * to an InvalidResponseException.
 *
 * `specText` is deliberately `requiredText` (present-but-empty fails
 * anyway because the spec is never empty, but the semantic point is that
 * it is DATA — the verbatim document, not an optional garnish). Counts
 * are `requiredNumber` integers validated ≥ 0; `specURL` and method
 * vocabularies are checked against the server's closed sets so a
 * drifting builder fails here first.
 */

export type DocsLinkDto = DocsLinkDemoData;

const HTTP_METHODS: readonly string[] = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'];

/** Deserialize + validate the GET /api/demo/api/docs-link payload. */
export function docsLinkDtoFromJson(body: unknown): DocsLinkDto {
  if (!isRecord(body)) throw new Error('docs-link payload must be an object');
  return {
    spec: specMetaFromJson(body.spec),
    operations: requiredArray(body, 'operations').map(operationFromJson),
    schemas: requiredArray(body, 'schemas').map(schemaFromJson),
    serving: servingFromJson(body.serving),
    drift: requiredArray(body, 'drift').map(driftRowFromJson),
    specText: requiredText(body, 'specText'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toDocsLinkDemo(dto: DocsLinkDto): DocsLinkDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function specMetaFromJson(body: unknown): DocsLinkSpecMeta {
  if (!isRecord(body)) throw new Error('spec meta must be an object');
  const meta: DocsLinkSpecMeta = {
    title: requiredString(body, 'title'),
    version: requiredString(body, 'version'),
    openapiVersion: requiredString(body, 'openapiVersion'),
    specPath: requiredString(body, 'specPath'),
    specURL: requiredString(body, 'specURL'),
    servers: requiredArray(body, 'servers').map((url) => {
      if (typeof url !== 'string' || url === '') {
        throw new Error('expected a non-empty server URL');
      }
      return url;
    }),
    pathCount: countFromJson(body, 'pathCount'),
    operationCount: countFromJson(body, 'operationCount'),
    schemaCount: countFromJson(body, 'schemaCount'),
    validationIssue: requiredString(body, 'validationIssue'),
  };
  if (meta.pathCount < 1 || meta.operationCount < 1 || meta.schemaCount < 1) {
    throw new Error('spec counts must describe a non-empty contract');
  }
  return meta;
}

function countFromJson(source: Record<string, unknown>, key: string): number {
  const value = requiredNumber(source, key);
  if (!Number.isInteger(value) || value < 0) {
    throw new Error(`expected a non-negative integer at "${key}"`);
  }
  return value;
}

function operationFromJson(body: unknown): DocsLinkOperation {
  if (!isRecord(body)) throw new Error('operation row must be an object');
  const method = requiredString(body, 'method');
  if (!HTTP_METHODS.includes(method)) {
    throw new Error(`unknown HTTP method "${method}"`);
  }
  return {
    id: requiredString(body, 'id'),
    method,
    path: requiredString(body, 'path'),
    summary: requiredText(body, 'summary'),
  };
}

function schemaFromJson(body: unknown): DocsLinkSchema {
  if (!isRecord(body)) throw new Error('schema row must be an object');
  return {
    name: requiredString(body, 'name'),
    description: requiredText(body, 'description'),
  };
}

function servingFromJson(body: unknown): DocsLinkServing {
  if (!isRecord(body)) throw new Error('serving story must be an object');
  return {
    specURL: requiredString(body, 'specURL'),
    contentType: requiredString(body, 'contentType'),
    servedVerbatim: requiredBoolean(body, 'servedVerbatim'),
    swaggerStory: requiredString(body, 'swaggerStory'),
  };
}

function driftRowFromJson(body: unknown): DocsLinkDriftRow {
  if (!isRecord(body)) throw new Error('drift row must be an object');
  return {
    check: requiredString(body, 'check'),
    compares: requiredString(body, 'compares'),
    settled: requiredString(body, 'settled'),
  };
}
