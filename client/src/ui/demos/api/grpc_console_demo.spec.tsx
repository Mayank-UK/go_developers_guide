import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { GRPCConsoleDemo } from './grpc_console_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetGRPCConsoleDemoUseCase,
  API_GRPC_CONSOLE_DEMO_USE_CASE,
} from '../../../domain/demo/get-grpc-console-demo-usecase';
import type { GRPCConsoleDemoData } from '../../../domain/demo/grpc-console';

/**
 * GRPCConsoleDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration instead
 * of stubbing fetch. Behavior, not implementation: the transport facts
 * render, one card per exhibit call renders with its kind, method path,
 * observed status code (name + numeric) and result rows — rejection rows
 * (NotFound, DeadlineExceeded, rejected acks) render as failures while
 * the call cards themselves stay truthful — and failures degrade to the
 * shared error panel with recovery.
 *
 * (The demo-registry binding test lands with the composition-root wiring —
 * this slice ships before demo-registry.tsx grows its lazy entry.)
 */

const fixture: GRPCConsoleDemoData = {
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
        { label: 'ack · p-7', detail: 'applied — tags now: go, rpc, streaming', ok: true },
        {
          label: 'ack · p-999',
          detail: 'rejected — Post "p-999" not found. — acked per item; the stream continues',
          ok: false,
        },
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
          label: 'observed status',
          detail: 'DeadlineExceeded (4) — deadlines propagate end to end; nothing waits out the work',
          ok: false,
        },
      ],
    },
  ],
};

function stubUseCase(execute: () => Promise<Result<GRPCConsoleDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    API_GRPC_CONSOLE_DEMO_USE_CASE,
    { execute } as unknown as GetGRPCConsoleDemoUseCase,
  );
}

describe('GRPCConsoleDemo', () => {
  it('renders loading first, then the transport facts and every call card', async () => {
    stubUseCase(async () => success(fixture));

    render(<GRPCConsoleDemo />);

    expect(screen.getByTestId('grpc-console-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('grpc-transport')).toBeInTheDocument());

    const transport = screen.getByTestId('grpc-transport');
    expect(transport).toHaveTextContent('bufconn listener');
    expect(transport).toHaveTextContent('passthrough:///bufnet');
    expect(transport).toHaveTextContent('guide.posts.v1.PostsService');
    expect(transport).toHaveTextContent('buf generate');

    // One card per call, tagged by kind.
    for (const kind of ['unary', 'server-stream', 'client-stream', 'bidi']) {
      expect(screen.getAllByTestId(`grpc-call-${kind}`).length).toBeGreaterThan(0);
    }
    expect(screen.getAllByTestId('grpc-call-unary')).toHaveLength(2); // the hit and the deadline case
  });

  it('shows status codes with their numeric values on every call card', async () => {
    stubUseCase(async () => success(fixture));

    render(<GRPCConsoleDemo />);

    await waitFor(() => expect(screen.getByTestId('grpc-call-bidi')).toBeInTheDocument());

    const chips = screen.getAllByTestId('grpc-status-chip').map((chip) => chip.textContent);
    expect(chips).toContain('OK (0)');
    expect(chips).toContain('DeadlineExceeded (4)');
  });

  it('renders rejection rows as failures while the method card stays truthful', async () => {
    stubUseCase(async () => success(fixture));

    render(<GRPCConsoleDemo />);

    await waitFor(() => expect(screen.getByTestId('grpc-call-bidi')).toBeInTheDocument());

    const bidi = screen.getByTestId('grpc-call-bidi');
    expect(bidi).toHaveTextContent('✗ ack · p-999');
    expect(bidi).toHaveTextContent('rejected — Post "p-999" not found.');
    // The call itself succeeded: the chip is the OK vocabulary.
    expect(bidi).toHaveTextContent('OK (0)');

    const deadline = screen.getAllByTestId('grpc-call-unary')[1]!;
    expect(deadline).toHaveTextContent('DeadlineExceeded (4)');
    expect(deadline).toHaveTextContent('✗ observed status');
  });

  it('shows the round-trip count and full method paths', async () => {
    stubUseCase(async () => success(fixture));

    render(<GRPCConsoleDemo />);

    await waitFor(() => expect(screen.getByTestId('grpc-call-server-stream')).toBeInTheDocument());

    const list = screen.getByTestId('grpc-call-server-stream');
    expect(list).toHaveTextContent('/guide.posts.v1.PostsService/ListPosts');
    expect(list).toHaveTextContent('2 round trips');
    expect(list).toHaveTextContent('cursor after page 1');
  });

  it('degrades to the shared error panel on a Failed Result, with recovery', async () => {
    let calls = 0;
    stubUseCase(async () => {
      calls += 1;
      return calls === 1
        ? failed(
            new ServerFailure('The demo endpoint is down.', {
              code: 'service_unavailable',
              status: 503,
              requestId: 'req-31',
            }),
          )
        : success(fixture);
    });

    render(<GRPCConsoleDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the whole in-process round trip and the console renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('grpc-transport')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
