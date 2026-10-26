import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { PipelineDemo } from './pipeline_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetMiddlewarePipelineDemoUseCase,
  MIDDLEWARE_PIPELINE_DEMO_USE_CASE,
} from '../../../domain/demo/get-middleware-pipeline-demo-usecase';
import type { MiddlewarePipelineDemoData } from '../../../domain/demo/middleware-pipeline';

/**
 * PipelineDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration instead
 * of stubbing fetch. Behavior, not implementation: the chain renders
 * outermost-first with the layers it mirrors, the numbered trace renders
 * with the pre-assignment beat shown as data ("not yet assigned"), the
 * request-id beat shows one id everywhere, the panic beat shows the
 * converted 500 and its captured log line, the timeout beat shows the
 * 30ms-vs-120ms arithmetic and the joined goroutine, and failures degrade
 * to the shared error panel with recovery.
 *
 * (The demo-registry binding test lands with the composition-root wiring —
 * this slice ships before demo-registry.tsx grows its lazy entry.)
 */

const fixture: MiddlewarePipelineDemoData = {
  chain: [
    { name: 'request-id', position: 0, role: 'assigns or honors one id', mirrors: 'presentation.RequestID' },
    { name: 'request-logger', position: 1, role: 'one line per request', mirrors: 'presentation.RequestLogger' },
    { name: 'recover', position: 2, role: 'panics to 500 envelopes', mirrors: 'presentation.Recover' },
    { name: 'terminal handler', position: 3, role: 'the endpoint', mirrors: 'http.ServeMux' },
  ],
  trace: [
    { layer: 'request-id', phase: 'before', sequence: 0, requestIDSeen: '', note: 'not yet assigned' },
    { layer: 'request-logger', phase: 'before', sequence: 1, requestIDSeen: 'middleware-demo-42', note: 'can cite the id' },
    { layer: 'recover', phase: 'before', sequence: 2, requestIDSeen: 'middleware-demo-42', note: 'recover armed' },
    { layer: 'terminal handler', phase: 'handler', sequence: 3, requestIDSeen: 'middleware-demo-42', note: 'ran exactly once' },
    { layer: 'recover', phase: 'after', sequence: 4, requestIDSeen: 'middleware-demo-42', note: 'unwinds' },
    { layer: 'request-logger', phase: 'after', sequence: 5, requestIDSeen: 'middleware-demo-42', note: 'logged at completion' },
    { layer: 'request-id', phase: 'after', sequence: 6, requestIDSeen: 'middleware-demo-42', note: 'header stamped early' },
  ],
  requestSideOrder: ['request-id', 'request-logger', 'recover', 'terminal handler'],
  responseSideOrder: ['terminal handler', 'recover', 'request-logger', 'request-id'],
  requestID: {
    headerName: 'x-request-id',
    inbound: 'middleware-demo-42',
    seenAtLayers: ['terminal handler (handler)', 'request-id (after)'],
    responseHeader: 'middleware-demo-42',
    sameIDEverywhere: true,
  },
  panicRecovery: {
    path: '/api/demo/middleware/pipeline',
    cause: 'assignment to entry in nil map',
    status: 500,
    body: '{"error":{"code":"internal"}}\n',
    loggedLine: 'level=ERROR msg="panic recovered" request_id=middleware-demo-42 panic="assignment to entry in nil map"',
    requestIDKept: true,
    trace: [],
  },
  timeoutBeat: {
    wrapper: 'http.TimeoutHandler',
    budgetMs: 30,
    handlerSleepMs: 120,
    observedStatus: 503,
    body: '<html><h1>Timeout</h1></html>',
    slowHandlerFinished: true,
    shape: 'the wrapper answers 503 itself.',
  },
  reasoning: 'Wrap order is request order; response order is its reverse.',
};

function stubUseCase(execute: () => Promise<Result<MiddlewarePipelineDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    MIDDLEWARE_PIPELINE_DEMO_USE_CASE,
    { execute } as unknown as GetMiddlewarePipelineDemoUseCase,
  );
}

describe('PipelineDemo', () => {
  it('renders loading first, then the full pipeline', async () => {
    stubUseCase(async () => success(fixture));

    render(<PipelineDemo />);

    expect(screen.getByTestId('middleware-pipeline-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('mp-chain')).toBeInTheDocument());

    // The chain, outermost first, mirroring the production wiring.
    const chain = screen.getByTestId('mp-chain');
    expect(chain).toHaveTextContent('request-id');
    expect(chain).toHaveTextContent('presentation.RequestID');
    expect(chain).toHaveTextContent('presentation.Recover');
    expect(chain).toHaveTextContent('http.ServeMux');
    expect(chain).toHaveTextContent('request: request-id → request-logger → recover → terminal handler');
    expect(chain).toHaveTextContent('response: terminal handler → recover → request-logger → request-id');

    // The numbered trace: the handler beat is in the middle; the FIRST
    // beat's missing id renders as data, not as a blank cell.
    const trace = screen.getByTestId('mp-trace');
    expect(trace).toHaveTextContent('terminal handler');
    expect(trace).toHaveTextContent('handler');
    expect(trace).toHaveTextContent('not yet assigned');
    expect(trace).toHaveTextContent('middleware-demo-42');

    // One id, everywhere — inbound and echoed.
    const requestId = screen.getByTestId('mp-request-id');
    expect(requestId).toHaveTextContent('x-request-id: middleware-demo-42');
    expect(requestId).toHaveTextContent('Every layer past the first observed the same id');

    // The converted panic: cause, 500, captured log line.
    const panic = screen.getByTestId('mp-panic');
    expect(panic).toHaveTextContent('assignment to entry in nil map');
    expect(panic).toHaveTextContent('500');
    expect(panic).toHaveTextContent('msg="panic recovered"');
    expect(panic).toHaveTextContent('request id intact');

    // The timeout beat: arithmetic, 503, joined goroutine.
    const timeout = screen.getByTestId('mp-timeout');
    expect(timeout).toHaveTextContent('budget 30ms vs sleep 120ms → 503');
    expect(timeout).toHaveTextContent('no leak');

    expect(screen.getByTestId('mp-reasoning')).toHaveTextContent('Wrap order is request order');
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

    render(<PipelineDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the pipeline renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('mp-chain')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
