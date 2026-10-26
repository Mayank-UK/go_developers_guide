import { describe, expect, it } from 'vitest';
import { grpcConsoleDtoFromJson, toGRPCConsoleDemo } from './grpc-console-dto';
import type { GRPCConsoleDemoData } from '../../domain/demo/grpc-console';

/**
 * GRPCConsoleDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). The fixture
 * mirrors the Go builder's REAL output (demos.BuildGRPCConsolePayload)
 * value-for-value — transport facts, the six exhibit calls, the observed
 * status codes — so any server-side drift in the exhibit script fails
 * here first.
 */

/** The real payload as the Go builder emits it (bufconn round trips). */
const realPayload = {
  transport: {
    mode: 'in-process bufconn listener — real HTTP/2 RPCs, no TCP port',
    dialTarget: 'passthrough:///bufnet',
    security: 'plaintext credentials, explicit — grpc refuses implicit insecure',
    protoFile: 'server/api/proto/posts.proto',
    service: 'guide.posts.v1.PostsService',
    generatedGo: 'server/internal/grpcapi/pb',
    codegenCmd: 'buf generate',
  },
  calls: [
    {
      kind: 'unary',
      method: '/guide.posts.v1.PostsService/GetPost',
      pattern: '1 request → 1 response',
      summary: 'GetPost("p-3") returned the full post — one message in, one message out',
      statusCode: 'OK',
      statusNumber: 0,
      roundTrips: 1,
      results: [
        { label: 'request', detail: 'PostRequest{Id: "p-3"}', ok: true },
        { label: 'response', detail: '"Errors are values" · tags: go, errors', ok: true },
      ],
    },
    {
      kind: 'unary',
      method: '/guide.posts.v1.PostsService/GetPost',
      pattern: '1 request → 1 response',
      summary:
        'GetPost("p-404") answered a status, not a post — and that is the whole error model',
      statusCode: 'NotFound',
      statusNumber: 5,
      roundTrips: 1,
      results: [
        { label: 'request', detail: 'PostRequest{Id: "p-404"}', ok: false },
        { label: 'observed status', detail: 'NotFound (5): Post "p-404" not found.', ok: false },
      ],
    },
    {
      kind: 'server-stream',
      method: '/guide.posts.v1.PostsService/ListPosts',
      pattern: '1 request → a stream of items',
      summary:
        'ListPosts(tag: "go", page_size: 3) walked by page token — four seeds match, so page 2 returns the partial remainder and no cursor',
      statusCode: 'OK',
      statusNumber: 0,
      roundTrips: 2,
      results: [
        { label: 'request · page 1', detail: 'ListRequest{Tag: "go", PageSize: 3}', ok: true },
        { label: 'page 1 received', detail: '3 items: p-6, p-3, p-2', ok: true },
        {
          label: 'cursor after page 1',
          detail: '"2" — opaque by contract: hand it back unmodified as the next PageToken',
          ok: true,
        },
        { label: 'page 2 received', detail: '1 item: p-1', ok: true },
        {
          label: 'cursor after page 2',
          detail: 'empty — the collection is exhausted; no more pages exist',
          ok: true,
        },
      ],
    },
    {
      kind: 'client-stream',
      method: '/guide.posts.v1.PostsService/CreatePosts',
      pattern: 'a stream of requests → 1 response',
      summary:
        'CreatePosts streamed three drafts and answered once — the summary carries received, created, and the ids in arrival order',
      statusCode: 'OK',
      statusNumber: 0,
      roundTrips: 1,
      results: [
        { label: 'messages sent', detail: '3 CreatePostRequest drafts on one stream', ok: true },
        { label: 'summary received', detail: 'received 3 · created 3', ok: true },
        { label: 'ids, in arrival order', detail: 'p-7, p-8, p-9', ok: true },
      ],
    },
    {
      kind: 'bidi',
      method: '/guide.posts.v1.PostsService/SyncTags',
      pattern: 'a stream of requests ⇄ a stream of responses',
      summary:
        'SyncTags took three updates and acked each one on the same call — the unknown post drew a rejection ack, and the stream kept going',
      statusCode: 'OK',
      statusNumber: 0,
      roundTrips: 1,
      results: [
        { label: 'messages sent', detail: '3 TagUpdates on one call, then CloseSend', ok: true },
        { label: 'ack · p-7', detail: 'applied — tags now: go, rpc, streaming', ok: true },
        {
          label: 'ack · p-999',
          detail: 'rejected — Post "p-999" not found. — acked per item; the stream continues',
          ok: false,
        },
        { label: 'ack · p-8', detail: 'applied — tags now: grpc, bidi', ok: true },
      ],
    },
    {
      kind: 'unary',
      method: '/guide.posts.v1.PostsService/GetPost',
      pattern: '1 request → 1 response, under a deadline',
      summary:
        'A 150ms deadline on the slow-path id — the server races its work against ctx.Done() and the deadline wins',
      statusCode: 'DeadlineExceeded',
      statusNumber: 4,
      roundTrips: 1,
      results: [
        {
          label: 'request',
          detail: 'PostRequest{Id: "p-slow"} · context.WithTimeout(150ms)',
          ok: true,
        },
        {
          label: 'observed status',
          detail: 'DeadlineExceeded (4) — deadlines propagate end to end; nothing waits out the work',
          ok: false,
        },
      ],
    },
  ],
} satisfies GRPCConsoleDemoData;

describe('grpcConsoleDtoFromJson', () => {
  it('parses the real builder output round-trip', () => {
    expect(grpcConsoleDtoFromJson(realPayload)).toEqual(realPayload);
  });

  it('keeps an empty detail as data (present-but-empty is not absence)', () => {
    const parsed = grpcConsoleDtoFromJson({
      ...realPayload,
      calls: [
        {
          ...realPayload.calls[0]!,
          results: [{ label: 'request', detail: '', ok: true }],
        },
      ],
    });

    expect(parsed.calls[0]?.results[0]?.detail).toBe('');
  });

  it.each([
    ['null', null],
    ['an array', [realPayload]],
    ['a string', 'nope'],
    ['missing transport', { ...realPayload, transport: undefined }],
    ['missing calls', { ...realPayload, calls: undefined }],
    ['calls not an array', { ...realPayload, calls: 'nope' }],
    ['an empty calls array', { ...realPayload, calls: [] }],
    ['a non-object call report', { ...realPayload, calls: ['nope'] }],
    ['missing kind', { ...realPayload, calls: [{ ...realPayload.calls[0]!, kind: undefined }] }],
    ['unknown kind vocabulary', { ...realPayload, calls: [{ ...realPayload.calls[0]!, kind: 'trinary' }] }],
    ['missing statusCode', { ...realPayload, calls: [{ ...realPayload.calls[0]!, statusCode: undefined }] }],
    ['unknown status vocabulary', { ...realPayload, calls: [{ ...realPayload.calls[0]!, statusCode: 'MAYBE' }] }],
    ['missing method', { ...realPayload, calls: [{ ...realPayload.calls[0]!, method: undefined }] }],
    ['missing statusNumber', { ...realPayload, calls: [{ ...realPayload.calls[0]!, statusNumber: undefined }] }],
    ['missing roundTrips', { ...realPayload, calls: [{ ...realPayload.calls[0]!, roundTrips: undefined }] }],
    ['missing results', { ...realPayload, calls: [{ ...realPayload.calls[0]!, results: undefined }] }],
    ['a non-object result row', { ...realPayload, calls: [{ ...realPayload.calls[0]!, results: ['x'] }] }],
    ['missing ok flag', { ...realPayload, calls: [{ ...realPayload.calls[0]!, results: [{ label: 'l', detail: 'd' }] }] }],
    ['missing dialTarget', { ...realPayload, transport: { ...realPayload.transport, dialTarget: undefined } }],
    ['missing service', { ...realPayload, transport: { ...realPayload.transport, service: undefined } }],
  ])('throws for %s', (_label, body) => {
    expect(() => grpcConsoleDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = grpcConsoleDtoFromJson({
      ...realPayload,
      extra: 'ignored',
      calls: realPayload.calls.map((call) => ({
        ...call,
        sneaky: true,
        results: call.results.map((row) => ({ ...row, extra: 1 })),
      })),
    });

    expect(parsed).toEqual(realPayload);
  });
});

describe('toGRPCConsoleDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toGRPCConsoleDemo(realPayload)).toBe(realPayload);
  });
});
