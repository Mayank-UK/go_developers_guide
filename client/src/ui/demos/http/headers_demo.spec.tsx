import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { HeadersDemo } from './headers_demo';
import {
  container,
} from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetHttpHeadersDemoUseCase,
  HTTP_HEADERS_DEMO_USE_CASE,
} from '../../../domain/demo/get-http-headers-demo-usecase';
import {
  REDACTED_HEADER_VALUE,
  type HttpHeadersDemoData,
} from '../../../domain/demo/http-headers';

/**
 * HeadersDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration instead
 * of stubbing fetch. Behavior, not implementation: the echoed request
 * line renders, classifications display per row, redacted credentials
 * render as withheld (never as a leaked value), server notes render, the
 * hop-by-hop reference renders, and failures degrade to the shared error
 * panel with recovery.
 *
 * (The demo-registry binding test lands with the composition-root wiring —
 * this slice ships before demo-registry.tsx grows its lazy entry.)
 */

const fixture: HttpHeadersDemoData = {
  method: 'GET',
  path: '/api/demo/http/headers',
  query: 'verbose=1',
  proto: 'HTTP/1.1',
  requestID: 'client-42',
  headers: [
    {
      name: 'User-Agent',
      value: 'curl/8.7.1',
      scope: 'end-to-end',
      role: 'identity',
      note: 'identifies the calling software; log it, rarely branch on it',
    },
    { name: 'Accept', value: 'application/json', scope: 'end-to-end', role: 'negotiation' },
    { name: 'Authorization', value: REDACTED_HEADER_VALUE, scope: 'end-to-end', role: 'security' },
    { name: 'Connection', value: 'keep-alive', scope: 'hop-by-hop', role: 'transport' },
    { name: 'X-Custom', value: 'teaching-value', scope: 'end-to-end', role: 'extension' },
  ],
  hopByHop: [
    'Connection',
    'Keep-Alive',
    'Proxy-Authenticate',
    'Proxy-Authorization',
    'Te',
    'Trailer',
    'Transfer-Encoding',
    'Upgrade',
  ],
};

function stubUseCase(execute: () => Promise<Result<HttpHeadersDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    HTTP_HEADERS_DEMO_USE_CASE,
    { execute } as unknown as GetHttpHeadersDemoUseCase,
  );
}

describe('HeadersDemo', () => {
  it('renders loading first, then the full echo', async () => {
    stubUseCase(async () => success(fixture));

    render(<HeadersDemo />);

    expect(screen.getByTestId('http-headers-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('hh-request-line')).toBeInTheDocument());

    // The request line, exactly as routed: method, path, query, protocol.
    expect(screen.getByTestId('hh-request-line')).toHaveTextContent('GET /api/demo/http/headers');
    expect(screen.getByTestId('hh-request-line')).toHaveTextContent('?verbose=1');
    expect(screen.getByTestId('hh-request-line')).toHaveTextContent('HTTP/1.1');
    expect(screen.getByTestId('hh-request-line')).toHaveTextContent('client-42');

    // Every inbound header with its two classifications.
    const table = screen.getByTestId('hh-table');
    expect(table).toHaveTextContent('User-Agent');
    expect(table).toHaveTextContent('end-to-end');
    expect(table).toHaveTextContent('hop-by-hop');
    expect(table).toHaveTextContent('negotiation');
    expect(table).toHaveTextContent('transport');
    expect(table).toHaveTextContent('extension');

    // Credentials: presence shown, value withheld — never the token itself.
    expect(table).toHaveTextContent(REDACTED_HEADER_VALUE);
    expect(table).toHaveTextContent('security');

    // Teaching notes ride under the table, one line per noted header.
    expect(screen.getByTestId('hh-notes')).toHaveTextContent('rarely branch on it');

    // The RFC's closed hop-by-hop set renders as the reference list.
    expect(screen.getByTestId('hh-hop-by-hop')).toHaveTextContent('Transfer-Encoding');
    expect(screen.getByTestId('hh-hop-by-hop')).toHaveTextContent('Upgrade');
  });

  it('omits the notes section when no header carried a note', async () => {
    stubUseCase(async () =>
      success({
        ...fixture,
        headers: fixture.headers.map((header) => ({ ...header, note: undefined })),
      }),
    );

    render(<HeadersDemo />);

    await waitFor(() => expect(screen.getByTestId('hh-table')).toBeInTheDocument());
    expect(screen.queryByTestId('hh-notes')).not.toBeInTheDocument();
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

    render(<HeadersDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the echo renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('hh-request-line')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
