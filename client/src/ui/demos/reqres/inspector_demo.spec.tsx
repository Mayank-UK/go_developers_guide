import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ReqresInspectorDemo } from './inspector_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetReqresInspectorDemoUseCase,
  REQRES_INSPECTOR_DEMO_USE_CASE,
} from '../../../domain/demo/get-reqres-inspector-demo-usecase';
import type { ReqresInspectorDemoData } from '../../../domain/demo/reqres-inspector';

/**
 * ReqresInspectorDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the request facts render, the discipline rules render, every exhibit
 * row shows its verdict with the error verbatim (and empty decoded
 * state stays visible — truncation decodes nothing), the body-less live
 * beat states absence, and failures degrade to the shared error panel
 * with recovery.
 *
 * (The demo-registry binding test lands with the composition-root
 * wiring — this slice ships before demo-registry.tsx grows its lazy
 * entry.)
 */

const fixture: ReqresInspectorDemoData = {
  request: {
    method: 'GET',
    path: '/api/demo/reqres/inspector',
    bodyBytes: 0,
    bodyLimit: 4096,
  },
  discipline: {
    decoder: 'json.NewDecoder over the request body',
    unknownFields: 'rejected — DisallowUnknownFields',
    trailing: 'exactly one JSON value — a second Decode must return io.EOF',
    firstError: 'reported verbatim; decoding stops at the first offense',
  },
  exhibits: [
    {
      label: 'valid body',
      body: '{"title":"Read the body as a stream","tags":["request","json"]}',
      outcome: 'accepted',
      decoded: 'title="Read the body as a stream" tags=["request" "json"]',
    },
    {
      label: 'unknown field, then a type error — first error wins',
      body: '{"title":"Valid title","urgent":true,"tags":"not-a-list"}',
      outcome: 'rejected',
      error: 'json: unknown field "urgent"',
      errorType: '*errors.errorString',
      decoded: 'title="Valid title" tags=[]',
    },
    {
      label: 'truncated mid-array',
      body: '{"title":"complete","tags":["a"',
      outcome: 'rejected',
      error: 'unexpected EOF',
      errorType: '*errors.errorString',
      unexpectedEOF: true,
      decoded: 'title="" tags=[]',
    },
    {
      label: 'wrong-typed field',
      body: '{"title":42,"tags":[]}',
      outcome: 'rejected',
      error: 'json: cannot unmarshal number into Go struct field reqresBody.title of type string',
      errorType: '*json.UnmarshalTypeError',
      field: 'title',
      decoded: 'title="" tags=[]',
    },
  ],
  liveBody: { sent: false, outcome: 'none' },
};

function stubUseCase(execute: () => Promise<Result<ReqresInspectorDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    REQRES_INSPECTOR_DEMO_USE_CASE,
    { execute } as unknown as GetReqresInspectorDemoUseCase,
  );
}

describe('ReqresInspectorDemo', () => {
  it('renders loading first, then the full inspection', async () => {
    stubUseCase(async () => success(fixture));

    render(<ReqresInspectorDemo />);

    expect(screen.getByTestId('reqres-inspector-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('ri-request')).toBeInTheDocument());

    // The live request's own facts.
    expect(screen.getByTestId('ri-request')).toHaveTextContent('GET /api/demo/reqres/inspector');
    expect(screen.getByTestId('ri-request')).toHaveTextContent('0 / 4096 bytes');

    // The discipline, stated as rules.
    expect(screen.getByTestId('ri-discipline')).toHaveTextContent('DisallowUnknownFields');
    expect(screen.getByTestId('ri-discipline')).toHaveTextContent('io.EOF');

    // Every exhibit with its verdict; errors verbatim; empty decoded
    // state stays visible (truncation decodes nothing).
    const table = screen.getByTestId('ri-exhibits');
    expect(table).toHaveTextContent('accepted');
    expect(table).toHaveTextContent('rejected');
    expect(table).toHaveTextContent('json: unknown field "urgent"');
    expect(table).toHaveTextContent('*json.UnmarshalTypeError');
    expect(table).toHaveTextContent('(field: title)');
    expect(table).toHaveTextContent('title="" tags=[]');
    expect(table).toHaveTextContent('errors.Is(io.ErrUnexpectedEOF)');

    // The body-less live beat states absence.
    expect(screen.getByTestId('ri-live')).toHaveTextContent('none');
  });

  it('renders the live beat as a real verdict when a body was sent', async () => {
    stubUseCase(async () =>
      success({
        ...fixture,
        request: { ...fixture.request, contentType: 'application/json', bodyBytes: 38 },
        liveBody: {
          sent: true,
          body: '{"title":"sent along","tags":["live"]}',
          outcome: 'accepted',
          decoded: 'title="sent along" tags=["live"]',
        },
      }),
    );

    render(<ReqresInspectorDemo />);

    await waitFor(() => expect(screen.getByTestId('ri-live')).toBeInTheDocument());
    expect(screen.getByTestId('ri-live')).toHaveTextContent('accepted');
    expect(screen.getByTestId('ri-live')).toHaveTextContent('title="sent along" tags=["live"]');
    expect(screen.getByTestId('ri-request')).toHaveTextContent('(application/json)');
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

    render(<ReqresInspectorDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the inspection renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('ri-request')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
