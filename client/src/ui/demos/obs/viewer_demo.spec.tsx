import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ObsViewerDemo } from './viewer_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetObsViewerDemoUseCase,
  OBS_VIEWER_DEMO_USE_CASE,
} from '../../../domain/demo/get-obs-viewer-demo-usecase';
import type { ObsViewerDemoData } from '../../../domain/demo/obs-viewer';

/**
 * ObsViewerDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration
 * instead of stubbing fetch. Behavior, not implementation: the anatomy
 * renders its fields with notes, the level matrix renders each
 * severity's survival set, the replay renders both requests and their
 * captured lines with the shared id, and failures degrade to the
 * shared error panel with recovery.
 */

const fixture: ObsViewerDemoData = {
  anatomy: {
    line: '{"time":"2026-10-27T10:00:00Z","level":"INFO","msg":"http_request"}',
    fields: [
      { key: 'time', value: '"2026-10-27T10:00:00Z"', note: 'stamped by the handler' },
      { key: 'level', value: '"INFO"', note: 'the dial the line passed' },
      { key: 'request_id', value: '"rid-obs-1"', note: 'the correlation key' },
    ],
    note: 'the line above is verbatim what slog wrote',
  },
  levels: {
    rows: [
      { level: 'DEBUG', survivesAt: ['debug'], emittedBy: 'nothing today', note: 'measured' },
      { level: 'INFO', survivesAt: ['debug', 'info'], emittedBy: 'the heartbeat', note: 'measured' },
    ],
    gate: 'GDG_LOG_LEVEL (debug|info|warn|error)',
    note: 'the dial is asymmetric by design',
  },
  replay: {
    requests: [
      { label: 'the 200', request: 'GET /api/posts?limit=2&sort=newest', requestId: 'rid-obs-1', status: 200 },
      { label: 'the 404', request: 'GET /api/posts/obs-viewer-missing', requestId: 'rid-obs-1', status: 404 },
    ],
    lines: [
      {
        source: 'the 404 (its failure line)',
        level: 'WARN',
        msg: 'error response',
        requestId: 'rid-obs-1',
        raw: '{"level":"WARN","msg":"error response","request_id":"rid-obs-1"}',
      },
      {
        source: 'the 404 (its access line)',
        level: 'INFO',
        msg: 'http_request',
        requestId: 'rid-obs-1',
        raw: '{"level":"INFO","msg":"http_request","status":404}',
      },
    ],
    note: 'the warn and the access line share ONE id: rid-obs-1',
  },
  reasoning: 'the log is data',
};

function stubUseCase(execute: () => Promise<Result<ObsViewerDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    OBS_VIEWER_DEMO_USE_CASE,
    { execute } as unknown as GetObsViewerDemoUseCase,
  );
}

describe('ObsViewerDemo', () => {
  it('renders loading first, then the anatomy, the level matrix, and the correlated replay', async () => {
    stubUseCase(async () => success(fixture));

    render(<ObsViewerDemo />);

    expect(screen.getByTestId('obs-viewer-loading')).toBeInTheDocument();

    await waitForReady();
    const anatomy = screen.getByTestId('obs-viewer-anatomy');
    expect(anatomy).toHaveTextContent('http_request');
    expect(screen.getByTestId('obs-viewer-field-request_id')).toHaveTextContent('the correlation key');

    const info = screen.getByTestId('obs-viewer-level-info');
    expect(info).toHaveTextContent('survives at: debug, info');

    const replay = screen.getByTestId('obs-viewer-replay');
    expect(replay).toHaveTextContent('GET /api/posts?limit=2&sort=newest');
    expect(replay).toHaveTextContent('404');
    expect(screen.getByTestId('obs-viewer-line-1')).toHaveTextContent('error response');
    expect(screen.getByTestId('obs-viewer-line-1')).toHaveTextContent('rid-obs-1');
    expect(replay).toHaveTextContent('share ONE id: rid-obs-1');

    expect(screen.getByTestId('obs-viewer-reasoning')).toHaveTextContent('the log is data');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure('The slog viewer could not be reached.', {
      code: 'network_error',
      status: 0,
      requestId: 'rid-obs-down',
    });
    stubUseCase(async () => failed(failure));

    render(<ObsViewerDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('obs-viewer-ready')).toBeInTheDocument());
}
