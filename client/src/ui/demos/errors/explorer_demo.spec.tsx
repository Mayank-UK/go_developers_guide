import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ErrorsExplorerDemo } from './explorer_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import { stubApiFetch } from '../../../test/fetch_stub';
import {
  GetErrorsExplorerDemoUseCase,
  ERRORS_EXPLORER_DEMO_USE_CASE,
} from '../../../domain/demo/get-errors-explorer-demo-usecase';
import type { ErrorsExplorerDemoData } from '../../../domain/demo/errors-explorer';

/**
 * ErrorsExplorerDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not
 * implementation: the embedded scenario control, the live walk with its
 * observed status and derived note, the taxonomy rows with their
 * triggers (try-it vs recipe), the scenario and probe panels, and
 * failures degrade to the shared error panel with recovery.
 */

const fixture: ErrorsExplorerDemoData = {
  kinds: [
    {
      kind: 'not_found',
      httpStatus: 404,
      defaultMessage: 'Resource not found.',
      raisedBy: 'use cases and handlers raise it directly',
      loggedAs: 'error response · warn — the plain miss, logged like every other kind',
      trigger: {
        mode: 'live',
        method: 'GET',
        path: '/api/posts/no-such-post',
        note: 'a well-formed id that names nothing — the plain 404 envelope',
      },
    },
    {
      kind: 'conflict',
      httpStatus: 409,
      defaultMessage: 'Resource conflict.',
      raisedBy: 'reserved for genuine state conflicts',
      loggedAs: 'error response · warn — when it fires, it fires like every other kind',
      trigger: {
        mode: 'recipe',
        method: 'POST',
        path: '/api/posts',
        note: 'no live trigger: nothing in the guide\u2019s runtime conflicts yet',
      },
    },
  ],
  envelopeNote:
    'every failure this server produces answers the SAME envelope, and stamps the x-request-id header',
  scenario: {
    getEndpoint: '/api/demo/scenario',
    patchEndpoint: '/api/demo/scenario',
    modes: ['none', 'error', 'latency'],
    note: 'the runtime toggle every demo reads — PATCH it, probe it, PATCH it back',
  },
  probe: {
    probeEndpoint: '/api/demo/errors/probe',
    latencyMillis: 1500,
    healthyNote: 'with injectFailure=none the probe answers 200 healthy',
    injectedNote: 'with injectFailure=error the probe panics deliberately',
    latencyNote: 'with injectFailure=latency the probe sleeps out its 1500 ms injection',
  },
  walk: {
    injectFailure: 'error',
    status: 500,
    elapsedMillis: 4,
    requestID: 'rid-explorer-3',
    body: '{"error":{"code":"internal","message":"Internal server error.","requestId":"rid-explorer-3"}}',
    note: 'error injection honored: the probe panicked, the Recover layer answered the generic 500 envelope',
  },
  reasoning: 'eight kinds, one envelope, zero exceptions to either',
};

/** The embedded scenario panel's mount-time GET; stubbed so every render is deterministic. */
const scenario = { url: '/api/demo/scenario', body: { dataMode: 'fake', injectFailure: 'none' } };

function stubUseCase(execute: () => Promise<Result<ErrorsExplorerDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    ERRORS_EXPLORER_DEMO_USE_CASE,
    { execute } as unknown as GetErrorsExplorerDemoUseCase,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('ErrorsExplorerDemo', () => {
  it('renders loading first, then the control, the walk, the table, and the panels', async () => {
    stubUseCase(async () => success(fixture));
    stubApiFetch([scenario]);

    render(<ErrorsExplorerDemo />);

    expect(screen.getByTestId('errors-explorer-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('errors-explorer-ready')).toBeInTheDocument());

    // The embedded scenario control — the demo's own fake/live + failure
    // toggle, so the page drives standalone.
    expect(await screen.findByTestId('scenario-panel')).toBeInTheDocument();
    expect(screen.getByLabelText('Data mode')).toBeInTheDocument();
    expect(screen.getByLabelText('Inject failure')).toBeInTheDocument();

    // The live walk: observed status, the envelope body, the derived note.
    const walk = screen.getByTestId('errors-explorer-walk');
    expect(walk).toHaveTextContent('injectFailure=error');
    expect(walk).toHaveTextContent('500');
    expect(walk).toHaveTextContent('rid-explorer-3');
    expect(walk).toHaveTextContent('"code":"internal"');
    expect(walk).toHaveTextContent('the probe panicked');

    // The taxonomy rows: one live trigger, one honest recipe.
    const notFound = screen.getByTestId('errors-explorer-kind-not_found');
    expect(notFound).toHaveTextContent('404');
    expect(notFound).toHaveTextContent('try it');
    expect(notFound).toHaveTextContent('GET /api/posts/no-such-post');
    const conflict = screen.getByTestId('errors-explorer-kind-conflict');
    expect(conflict).toHaveTextContent('recipe');
    expect(conflict).toHaveTextContent('no live trigger');

    // The panels and the closing notes.
    expect(screen.getByTestId('errors-explorer-scenario')).toHaveTextContent(
      'none · error · latency',
    );
    expect(screen.getByTestId('errors-explorer-probe')).toHaveTextContent('1500 ms');
    expect(screen.getByTestId('errors-explorer-envelope')).toHaveTextContent('SAME envelope');
    expect(screen.getByTestId('errors-explorer-reasoning')).toHaveTextContent('zero exceptions');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure(
      'The errors explorer could not be reached.',
      { code: 'network_error', status: 0, requestId: 'rid-explorer-down' },
    );
    stubUseCase(async () => failed(failure));
    stubApiFetch([scenario]);

    render(<ErrorsExplorerDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('errors-explorer-ready')).toBeInTheDocument());
    // The recovered view re-mounts the embedded scenario control.
    expect(await screen.findByTestId('scenario-panel')).toBeInTheDocument();
  });
});
