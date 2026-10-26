import { describe, expect, it } from 'vitest';
import { graphqlExecutionFromJson, graphqlPlaygroundDtoFromJson } from './graphql-playground-dto';

/**
 * GraphQLPlaygroundDTO spec — the fixture below is the parsed output of
 * the server's demos.BuildGraphQLPlaygroundPayload builder
 * (internal/domain/demos/graphql_playground.go, dumped from the running
 * Go test suite), so this spec fails the moment the server's payload
 * drifts from what this DTO validates: wire drift becomes a red spec,
 * not a runtime surprise.
 */

// eslint-disable-next-line @typescript-eslint/quotes -- real wire bytes
const realPayload = JSON.parse(`{
  "endpoint": {
    "path": "/api/graphql",
    "method": "POST",
    "requestBody": "{\\"query\\":\\"{ posts(limit: 2) { items { id title } } }\\"}",
    "liveNote": "the route is wired at the composition root (cmd/guide/main.go, POST /api/graphql); these runs execute the SAME generated executor in-process over a scratch repository, so the payload is deterministic while the RUN button and the curl tour hit the live endpoint"
  },
  "schema": {
    "operations": [
      "query posts(page: Int, limit: Int, tag: String, sort: String): PostPage!",
      "query post(id: ID!): Post",
      "mutation createPost(input: PostInput!): Post!",
      "mutation updatePost(id: ID!, input: PostInput!): Post",
      "mutation deletePost(id: ID!): ID"
    ],
    "types": [
      {
        "name": "Post",
        "kind": "object",
        "fields": [
          { "name": "id", "type": "ID!" },
          { "name": "title", "type": "String!" },
          { "name": "body", "type": "String!" },
          { "name": "tags", "type": "[String!]!" },
          { "name": "createdAt", "type": "Time!" },
          { "name": "updatedAt", "type": "Time!" }
        ]
      },
      {
        "name": "PostPage",
        "kind": "object",
        "fields": [
          { "name": "items", "type": "[Post!]!" },
          { "name": "page", "type": "Int!" },
          { "name": "limit", "type": "Int!" },
          { "name": "totalItems", "type": "Int!" },
          { "name": "totalPages", "type": "Int!" },
          { "name": "hasNextPage", "type": "Boolean!" },
          { "name": "hasPrevPage", "type": "Boolean!" }
        ]
      },
      {
        "name": "PostInput",
        "kind": "input",
        "fields": [
          { "name": "title", "type": "String!" },
          { "name": "body", "type": "String!" },
          { "name": "tags", "type": "[String!]!" }
        ]
      }
    ]
  },
  "samples": [
    {
      "id": "list-first-page",
      "label": "List the first page (paginated)",
      "kind": "query",
      "query": "query { posts(page: 1, limit: 2, sort: \\"oldest\\") {\\n  items { id title }\\n  page limit totalItems totalPages hasNextPage hasPrevPage\\n} }",
      "note": "the page envelope, field-for-field the REST wire's shape"
    },
    {
      "id": "read-one",
      "label": "Read one post by id",
      "kind": "query",
      "query": "query { post(id: \\"p-3\\") { id title tags } }",
      "note": "nullable field: a miss answers null plus an errors entry"
    },
    {
      "id": "create-post",
      "label": "Create a post (mutation)",
      "kind": "mutation",
      "query": "mutation { createPost(input: {\\n  title: \\"GraphQL wrote this\\"\\n  body: \\"One endpoint, one document, the fields you asked for.\\"\\n  tags: [\\"graphql\\", \\"demo\\"]\\n}) { id title tags } }",
      "note": "the first scratch-world create is p-7, like the REST tour's"
    },
    {
      "id": "update-post",
      "label": "Full-replace that post (mutation)",
      "kind": "mutation",
      "query": "mutation { updatePost(id: \\"p-7\\", input: {\\n  title: \\"Edited through the graph\\"\\n  body: \\"The input is the whole next state — old tags are gone, not merged.\\"\\n  tags: [\\"edited\\"]\\n}) { id title tags } }",
      "note": "PUT semantics: omitting a field removes it"
    },
    {
      "id": "delete-post",
      "label": "Delete it (mutation, strict stance)",
      "kind": "mutation",
      "query": "mutation { deletePost(id: \\"p-7\\") }",
      "note": "answers the removed id; deleting a vanished post is a not_found error"
    },
    {
      "id": "error-not-found",
      "label": "A miss, the GraphQL way",
      "kind": "error",
      "query": "query { post(id: \\"p-999\\") { id title } }",
      "note": "200 with {data: {post: null}} and an errors entry — extensions.code is the same not_found the REST 404 envelope serves"
    },
    {
      "id": "error-validation",
      "label": "A rule violation, details and all",
      "kind": "error",
      "query": "query { posts(limit: 500) { totalItems } }",
      "note": "the same rules the REST list enforces; extensions.details carries the issue list the 422 serves"
    },
    {
      "id": "introspection",
      "label": "Ask the schema about itself",
      "kind": "introspection",
      "query": "query { __schema { queryType { name } mutationType { name } } }",
      "note": "how every GraphQL tool discovers the contract at runtime"
    }
  ],
  "runs": [
    {
      "id": "list-first-page",
      "label": "List the first page (paginated)",
      "query": "query { posts(page: 1, limit: 2, sort: \\"oldest\\") {\\n  items { id title }\\n  page limit totalItems totalPages hasNextPage hasPrevPage\\n} }",
      "data": {
        "posts": {
          "items": [
            { "id": "p-1", "title": "Why Go for backends" },
            { "id": "p-2", "title": "ServeMux patterns in Go 1.22" }
          ],
          "page": 1,
          "limit": 2,
          "totalItems": 6,
          "totalPages": 3,
          "hasNextPage": true,
          "hasPrevPage": false
        }
      },
      "errors": [],
      "notes": [
        "runs against the deterministic six-post seed; oldest-first order is pinned by fixed January 2026 stamps"
      ]
    },
    {
      "id": "read-one",
      "label": "Read one post by id",
      "query": "query { post(id: \\"p-3\\") { id title tags } }",
      "data": {
        "post": { "id": "p-3", "title": "Errors are values", "tags": ["go", "errors"] }
      },
      "errors": [],
      "notes": []
    },
    {
      "id": "create-post",
      "label": "Create a post (mutation)",
      "query": "mutation { createPost(input: {\\n  title: \\"GraphQL wrote this\\"\\n  body: \\"One endpoint, one document, the fields you asked for.\\"\\n  tags: [\\"graphql\\", \\"demo\\"]\\n}) { id title tags } }",
      "data": {
        "createPost": { "id": "p-7", "title": "GraphQL wrote this", "tags": ["graphql", "demo"] }
      },
      "errors": [],
      "notes": [
        "the repository stamps createdAt/updatedAt live, so the selection pins the deterministic fields"
      ]
    },
    {
      "id": "update-post",
      "label": "Full-replace that post (mutation)",
      "query": "mutation { updatePost(id: \\"p-7\\", input: {\\n  title: \\"Edited through the graph\\"\\n  body: \\"The input is the whole next state — old tags are gone, not merged.\\"\\n  tags: [\\"edited\\"]\\n}) { id title tags } }",
      "data": {
        "updatePost": { "id": "p-7", "title": "Edited through the graph", "tags": ["edited"] }
      },
      "errors": [],
      "notes": []
    },
    {
      "id": "delete-post",
      "label": "Delete it (mutation, strict stance)",
      "query": "mutation { deletePost(id: \\"p-7\\") }",
      "data": { "deletePost": "p-7" },
      "errors": [],
      "notes": []
    },
    {
      "id": "error-not-found",
      "label": "A miss, the GraphQL way",
      "query": "query { post(id: \\"p-999\\") { id title } }",
      "data": { "post": null },
      "errors": [
        {
          "message": "Post \\"p-999\\" not found.",
          "extensions": { "code": "not_found" }
        }
      ],
      "notes": []
    },
    {
      "id": "error-validation",
      "label": "A rule violation, details and all",
      "query": "query { posts(limit: 500) { totalItems } }",
      "data": null,
      "errors": [
        {
          "message": "Invalid post list query.",
          "extensions": {
            "code": "validation_failed",
            "details": [
              { "field": "limit", "rule": "an integer 1–50 when sent; omit for 10", "got": "\\"500\\"" }
            ]
          }
        }
      ],
      "notes": []
    },
    {
      "id": "introspection",
      "label": "Ask the schema about itself",
      "query": "query { __schema { queryType { name } mutationType { name } } }",
      "data": {
        "__schema": {
          "queryType": { "name": "Query" },
          "mutationType": { "name": "Mutation" }
        }
      },
      "errors": [],
      "notes": []
    }
  ],
  "loader": {
    "postCount": 6,
    "loadsIssued": 12,
    "duplicateLoads": 6,
    "uniqueIds": 6,
    "naivePortCalls": 7,
    "naiveBreakdown": { "list": 1, "getById": 6, "create": 0, "replace": 0, "delete": 0 },
    "loaderPortCalls": 1,
    "loaderBreakdown": { "list": 1, "getById": 0, "create": 0, "replace": 0, "delete": 0 },
    "allResolved": true,
    "verdict": "12 concurrent loads (6 duplicates) crossed the port ONCE, batched — against 7 port calls for the naive one-read-per-item hydration; behind a database every port call is a round trip"
  }
}`);

describe('graphqlPlaygroundDtoFromJson (against the real builder output)', () => {
  it('validates the server payload, every field and closed vocabulary', () => {
    const dto = graphqlPlaygroundDtoFromJson(realPayload);

    expect(dto.endpoint.path).toBe('/api/graphql');
    expect(dto.endpoint.method).toBe('POST');
    expect(dto.endpoint.requestBody).toContain('"query"');

    expect(dto.schema.operations).toHaveLength(5);
    expect(dto.schema.operations[0]).toBe(
      'query posts(page: Int, limit: Int, tag: String, sort: String): PostPage!',
    );
    const byName = new Map(dto.schema.types.map((type) => [type.name, type]));
    expect(byName.get('Post')?.fields).toHaveLength(6);
    expect(byName.get('PostPage')?.fields).toHaveLength(7);
    expect(byName.get('PostInput')?.kind).toBe('input');

    expect(dto.samples).toHaveLength(8);
    expect(dto.samples.map((sample) => sample.kind)).toEqual([
      'query',
      'query',
      'mutation',
      'mutation',
      'mutation',
      'error',
      'error',
      'introspection',
    ]);

    expect(dto.runs).toHaveLength(8);
    expect(dto.samples.map((sample) => sample.id)).toEqual(dto.runs.map((run) => run.id));
  });

  it('keeps the executed runs honest: values, nulls, and extension codes', () => {
    const dto = graphqlPlaygroundDtoFromJson(realPayload);
    const runs = new Map(dto.runs.map((run) => [run.id, run]));

    const page = runs.get('list-first-page')?.data as { posts: { totalItems: number } } | null;
    expect(page?.posts.totalItems).toBe(6);

    // A nullable field answered null beside the error — null is a VALUE.
    expect(runs.get('error-not-found')?.data).toEqual({ post: null });
    expect(runs.get('error-not-found')?.errors[0]?.extensions?.['code']).toBe('not_found');

    // Execution failed before data assembled: data is the JSON null.
    expect(runs.get('error-validation')?.data).toBeNull();
    const violation = runs.get('error-validation')?.errors[0]?.extensions;
    expect(violation?.['code']).toBe('validation_failed');
    expect(Array.isArray(violation?.['details'])).toBe(true);

    expect(runs.get('introspection')?.errors).toEqual([]);
  });

  it('carries the loader experiment numbers the tests pin server-side', () => {
    const dto = graphqlPlaygroundDtoFromJson(realPayload);

    expect(dto.loader.naivePortCalls).toBe(7);
    expect(dto.loader.naiveBreakdown.getById).toBe(6);
    expect(dto.loader.loaderPortCalls).toBe(1);
    expect(dto.loader.loaderBreakdown.list).toBe(1);
    expect(dto.loader.allResolved).toBe(true);
  });

  it('throws on shape violations (missing runs, bad kind, null errors)', () => {
    const noRuns = { ...(realPayload as object), runs: undefined };
    expect(() => graphqlPlaygroundDtoFromJson(noRuns)).toThrow();

    const badKind = JSON.parse(JSON.stringify(realPayload)) as Record<string, unknown>;
    const samples = badKind['samples'] as Array<Record<string, unknown>>;
    samples[0]!['kind'] = 'subroutine';
    expect(() => graphqlPlaygroundDtoFromJson(badKind)).toThrow(/unknown sample kind/);

    const nullErrors = JSON.parse(JSON.stringify(realPayload)) as Record<string, unknown>;
    const runs = nullErrors['runs'] as Array<Record<string, unknown>>;
    runs[0]!['errors'] = null;
    expect(() => graphqlPlaygroundDtoFromJson(nullErrors)).toThrow();
  });
});

describe('graphqlExecutionFromJson (the live endpoint response)', () => {
  it('parses a clean result, an errors-with-null-field result, and a null-data result', () => {
    const clean = graphqlExecutionFromJson({
      data: { posts: { totalItems: 6 } },
      errors: [],
    });
    expect(clean.data).toEqual({ posts: { totalItems: 6 } });
    expect(clean.errors).toEqual([]);

    const miss = graphqlExecutionFromJson({
      data: { post: null },
      errors: [{ message: 'Post "p-999" not found.', extensions: { code: 'not_found' } }],
    });
    expect(miss.data).toEqual({ post: null });
    expect(miss.errors[0]?.extensions?.['code']).toBe('not_found');

    const preExecution = graphqlExecutionFromJson({
      data: null,
      errors: [{ message: 'Cannot query field "nosuchfield" on type "Query".' }],
    });
    expect(preExecution.data).toBeNull();
    expect(preExecution.errors).toHaveLength(1);
  });

  it('throws when the body is not the response document', () => {
    expect(() => graphqlExecutionFromJson([1, 2, 3])).toThrow();
    expect(() => graphqlExecutionFromJson({ errors: [] })).toThrow(/data member/);
    expect(() => graphqlExecutionFromJson({ data: 7 })).toThrow();
  });
});
