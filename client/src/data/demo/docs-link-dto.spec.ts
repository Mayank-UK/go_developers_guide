import { describe, expect, it } from 'vitest';
import { docsLinkDtoFromJson, toDocsLinkDemo } from './docs-link-dto';
import type { DocsLinkDemoData } from '../../domain/demo/docs-link';

/**
 * DocsLinkDTO spec — hostile-input discipline: only the exact wire shape
 * parses, anything else throws (and therefore surfaces upstream as an
 * invalid_response Failure, never as a guessed default). The fixture
 * mirrors the Go builder's real output (BuildAPIDocsLinkPayload): two
 * paths, five operations, six schemas, the serving story, the four drift
 * rows, and the verbatim spec text.
 */

const wellFormed: DocsLinkDemoData = {
  spec: {
    title: 'Go Developers Guide — posts API',
    version: '1.0.0',
    openapiVersion: '3.1.0',
    specPath: 'server/api/openapi.yaml',
    specURL: '/api/openapi.yaml',
    servers: ['http://localhost:8100'],
    pathCount: 2,
    operationCount: 5,
    schemaCount: 6,
    validationIssue: 'the drift test walks live JSON against these schemas — a spec that lies about the wire fails the suite',
  },
  operations: [
    { id: 'listPosts', method: 'GET', path: '/api/posts', summary: 'One page of the posts collection' },
    { id: 'createPost', method: 'POST', path: '/api/posts', summary: 'Create a post from a full draft body' },
    { id: 'getPost', method: 'GET', path: '/api/posts/{id}', summary: 'Read one post by id' },
    { id: 'replacePost', method: 'PUT', path: '/api/posts/{id}', summary: 'Full replacement — the body is the whole next state' },
    { id: 'deletePost', method: 'DELETE', path: '/api/posts/{id}', summary: 'Remove a post under the strict stance' },
  ],
  schemas: [
    { name: 'ErrorBody', description: "The envelope's payload: taxonomy bucket, human sentence, join key, structured context." },
    { name: 'ErrorEnvelope', description: 'Every failure on every route wears this one shape.' },
    { name: 'Post', description: 'One post on the wire — the response shape of read, create, and replace.' },
    { name: 'PostDraft', description: 'The create/replace request body — title and body required, tags optional.' },
    { name: 'PostPage', description: 'The list envelope — one page’s items plus the honest pagination metadata.' },
    { name: 'ValidationIssue', description: 'One validation finding, structured for the details array.' },
  ],
  serving: {
    specURL: '/api/openapi.yaml',
    contentType: 'application/yaml',
    servedVerbatim: true,
    swaggerStory: 'point Swagger UI at the URL and it renders the docs from the same bytes',
  },
  drift: [
    {
      check: 'list response ↔ PostPage',
      compares: 'GET /api/posts body: every key, type, and bound against the schema',
      settled: 'spec_drift_test.go drives the live handler and walks the JSON',
    },
    {
      check: 'single post ↔ Post',
      compares: 'GET /api/posts/{id} body: field names, tags never null, RFC 3339 timestamps',
      settled: 'the walker refuses unknown keys (additionalProperties: false)',
    },
  ],
  specText: 'openapi: 3.1.0\ninfo:\n  title: Go Developers Guide — posts API\n  version: 1.0.0\n',
};

describe('docsLinkDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(docsLinkDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps an empty summary as data (requiredText, not requiredString)', () => {
    const parsed = docsLinkDtoFromJson({
      ...wellFormed,
      operations: [{ id: 'listPosts', method: 'GET', path: '/api/posts', summary: '' }],
    });

    expect(parsed.operations[0]?.summary).toBe('');
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing spec', { ...wellFormed, spec: undefined }],
    ['spec not an object', { ...wellFormed, spec: 'nope' }],
    ['missing title', { ...wellFormed, spec: { ...wellFormed.spec, title: undefined } }],
    ['empty specPath', { ...wellFormed, spec: { ...wellFormed.spec, specPath: '' } }],
    ['servers not an array', { ...wellFormed, spec: { ...wellFormed.spec, servers: 'http://x' } }],
    ['a non-string server URL', { ...wellFormed, spec: { ...wellFormed.spec, servers: [42] } }],
    ['a fractional pathCount', { ...wellFormed, spec: { ...wellFormed.spec, pathCount: 1.5 } }],
    ['a negative schemaCount', { ...wellFormed, spec: { ...wellFormed.spec, schemaCount: -1 } }],
    ['a zero-operation contract', { ...wellFormed, spec: { ...wellFormed.spec, operationCount: 0 } }],
    ['missing operations', { ...wellFormed, operations: undefined }],
    ['a non-object operation row', { ...wellFormed, operations: ['nope'] }],
    ['an unknown HTTP method', { ...wellFormed, operations: [{ id: 'x', method: 'FETCH', path: '/api/posts', summary: 's' }] }],
    ['missing schema rows', { ...wellFormed, schemas: undefined }],
    ['a schema row without description', { ...wellFormed, schemas: [{ name: 'Post' }] }],
    ['serving missing', { ...wellFormed, serving: undefined }],
    ['servedVerbatim not boolean', { ...wellFormed, serving: { ...wellFormed.serving, servedVerbatim: 'yes' } }],
    ['drift missing', { ...wellFormed, drift: undefined }],
    ['a drift row missing settled', { ...wellFormed, drift: [{ check: 'c', compares: 'x' }] }],
    ['specText missing', { ...wellFormed, specText: undefined }],
    ['specText not a string', { ...wellFormed, specText: 42 }],
    ['null servers (null is not omission)', { ...wellFormed, spec: { ...wellFormed.spec, servers: null } }],
  ])('throws for %s', (_label, body) => {
    expect(() => docsLinkDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = docsLinkDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      operations: wellFormed.operations.map((operation) => ({ ...operation, sneaky: true })),
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toDocsLinkDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toDocsLinkDemo(wellFormed)).toBe(wellFormed);
  });
});
