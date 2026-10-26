import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ReqresMethodsDemo } from './methods_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetReqresMethodsDemoUseCase,
  REQRES_METHODS_DEMO_USE_CASE,
} from '../../../domain/demo/get-reqres-methods-demo-usecase';
import type { ReqresMethodsDemoData } from '../../../domain/demo/reqres-methods';

/**
 * ReqresMethodsDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the resource and Allow render, every observed method row shows its
 * status/phrase/headers/body (empty bodies render their rule — 204 and
 * friends carry none), the SSE beat renders its events and flush count,
 * and failures degrade to the shared error panel with recovery.
 *
 * (The demo-registry binding test lands with the composition-root
 * wiring — this slice ships before demo-registry.tsx grows its lazy
 * entry.)
 */

/** Mirrors the Go builder's real output, trimmed to four representative rows. */
const fixture: ReqresMethodsDemoData = {
  resource: '/api/demo/reqres/methods/exhibit',
  allow: 'GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE',
  methods: [
    {
      method: 'GET',
      status: 200,
      statusText: 'OK',
      headers: [
        { name: 'Content-Type', value: 'application/json' },
        { name: 'Cache-Control', value: 'no-store' },
        { name: 'Set-Cookie', value: 'reqres_visit=1; Path=/api/demo/reqres; HttpOnly; SameSite=Lax' },
        { name: 'X-Reqres-Method', value: 'GET' },
      ],
      bodyShape: "one JSON object — the resource's current state",
      body: '{"resource":"methods-exhibit","state":"steady"}',
    },
    {
      method: 'POST',
      status: 201,
      statusText: 'Created',
      headers: [
        { name: 'Content-Type', value: 'application/json' },
        { name: 'Location', value: '/api/demo/reqres/methods/exhibit/17' },
        { name: 'X-Reqres-Method', value: 'POST' },
      ],
      bodyShape: 'one JSON object — the created record; Location names where it lives',
      body: '{"id":17,"state":"created"}',
    },
    {
      method: 'PATCH',
      status: 204,
      statusText: 'No Content',
      headers: [{ name: 'X-Reqres-Method', value: 'PATCH' }],
      bodyShape: 'no body — 204 means the headers are the whole answer',
      body: '',
    },
    {
      method: 'TRACE',
      status: 405,
      statusText: 'Method Not Allowed',
      headers: [
        { name: 'Allow', value: 'GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE' },
        { name: 'X-Reqres-Method', value: 'TRACE' },
      ],
      bodyShape: 'one JSON object — the error note; Allow states what IS served',
      body: '{"error":"TRACE","served":"GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE"}',
    },
  ],
  sse: {
    contentType: 'text/event-stream',
    events: [
      { name: 'beat', data: '{"sequence":1,"state":"tick"}' },
      { name: 'beat', data: '{"sequence":2,"state":"tick"}' },
      { name: 'beat', data: '{"sequence":3,"state":"tick"}' },
    ],
    flushes: 3,
    framing: 'each event block ends with a blank line — the delimiter a client parser waits for',
  },
};

function stubUseCase(execute: () => Promise<Result<ReqresMethodsDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    REQRES_METHODS_DEMO_USE_CASE,
    { execute } as unknown as GetReqresMethodsDemoUseCase,
  );
}

describe('ReqresMethodsDemo', () => {
  it('renders loading first, then the full observed report', async () => {
    stubUseCase(async () => success(fixture));

    render(<ReqresMethodsDemo />);

    expect(screen.getByTestId('reqres-methods-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('rm-resource')).toBeInTheDocument());

    // The resource identity and the capability statement.
    expect(screen.getByTestId('rm-resource')).toHaveTextContent('/api/demo/reqres/methods/exhibit');
    expect(screen.getByTestId('rm-resource')).toHaveTextContent('Allow: GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE');

    // Every observed row: status + phrase, defining headers, exact bytes.
    const table = screen.getByTestId('rm-table');
    expect(table).toHaveTextContent('201 Created');
    expect(table).toHaveTextContent('Location: /api/demo/reqres/methods/exhibit/17');
    expect(table).toHaveTextContent('Set-Cookie: reqres_visit=1');
    expect(table).toHaveTextContent('405 Method Not Allowed');
    expect(table).toHaveTextContent('{"id":17,"state":"created"}');

    // Body-less rows render their rule — empty is the fact.
    expect(table).toHaveTextContent('no body — 204 means the headers are the whole answer');

    // The SSE beat: events, data payloads, flush count, framing.
    expect(screen.getByTestId('rm-sse')).toHaveTextContent('3 events, 3 flushes');
    expect(screen.getByTestId('rm-sse')).toHaveTextContent('text/event-stream');
    expect(screen.getByTestId('rm-sse')).toHaveTextContent('{"sequence":3,"state":"tick"}');
    expect(screen.getByTestId('rm-sse')).toHaveTextContent('blank line');
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

    render(<ReqresMethodsDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the report renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('rm-resource')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
