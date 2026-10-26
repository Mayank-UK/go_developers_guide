import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { ScenarioPanel } from './scenario-panel';
import { container } from '../../core/container';
import { createAppConfig } from '../../core/config/app-config';
import type { StubRoute } from '../../test/fetch_stub';

/**
 * ScenarioPanel spec — behavior through the real container + stubbed fetch:
 * mount load, PATCH-on-change with the partial body, revert-to-last-good on a
 * failed PATCH, the degraded unavailable state, the one-PATCH-in-flight
 * guarantee, and StrictMode's double mount never issuing a PATCH.
 */

beforeAll(() => {
  container.initialize(createAppConfig());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

interface RecordedCall {
  url: string;
  init: RequestInit | undefined;
}

/** stubApiFetch + a recorder, so rounds can be counted and bodies inspected. */
function stubRecordingFetch(routes: readonly StubRoute[]): RecordedCall[] {
  const calls: RecordedCall[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input.toString();
      const method = init?.method ?? 'GET';
      calls.push({ url, init });
      const route = routes.find(
        (candidate) =>
          candidate.url === url &&
          (candidate.method === undefined || candidate.method === method),
      );
      if (route?.delayMs !== undefined) {
        await new Promise((resolve) => setTimeout(resolve, route.delayMs));
      }
      const body =
        route?.body ?? { error: { code: 'not_found', message: `no stub for ${url}`, requestId: 'stub' } };
      const status = route?.status ?? (route === undefined ? 404 : 200);
      return { ok: status < 300, status, json: async () => body };
    }),
  );
  return calls;
}

/** Method-explicit so a PATCH round can never shadow-match the GET route. */
const fakeNone = {
  url: '/api/demo/scenario',
  method: 'GET',
  body: { dataMode: 'fake', injectFailure: 'none' },
};

function dataModeSelect(): HTMLSelectElement {
  return screen.getByLabelText('Data mode') as HTMLSelectElement;
}

function injectFailureSelect(): HTMLSelectElement {
  return screen.getByLabelText('Inject failure') as HTMLSelectElement;
}

describe('ScenarioPanel', () => {
  it('loads the snapshot on mount and renders both selects from it', async () => {
    stubRecordingFetch([
      { url: '/api/demo/scenario', body: { dataMode: 'live', injectFailure: 'error' } },
    ]);

    render(<ScenarioPanel />);

    expect(screen.getByTestId('scenario-loading')).toBeInTheDocument();
    expect(await screen.findByTestId('scenario-panel')).toBeInTheDocument();
    expect(dataModeSelect().value).toBe('live');
    expect(injectFailureSelect().value).toBe('error');
    expect(screen.queryByTestId('scenario-error')).not.toBeInTheDocument();
  });

  it('PATCHes only the changed key on user change and adopts the server answer', async () => {
    const calls = stubRecordingFetch([
      fakeNone,
      { url: '/api/demo/scenario', method: 'PATCH', body: { dataMode: 'fake', injectFailure: 'latency' } },
    ]);

    render(<ScenarioPanel />);
    await screen.findByTestId('scenario-panel');

    fireEvent.change(injectFailureSelect(), { target: { value: 'latency' } });

    await waitFor(() => expect(injectFailureSelect().value).toBe('latency'));
    // Let the round resolve, then confirm the SERVER answer stuck (not just
    // the optimistic value).
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 10));
    });
    expect(injectFailureSelect().value).toBe('latency');
    const patchCalls = calls.filter((call) => call.init?.method === 'PATCH');
    expect(patchCalls).toHaveLength(1);
    expect(JSON.parse(String(patchCalls[0]?.init?.body))).toEqual({
      injectFailure: 'latency',
    });
    expect(screen.queryByTestId('scenario-error')).not.toBeInTheDocument();
  });

  it('reverts the select to last-known-good and shows a brief inline error when the PATCH fails', async () => {
    stubRecordingFetch([
      fakeNone,
      {
        url: '/api/demo/scenario',
        method: 'PATCH',
        status: 503,
        body: { error: { code: 'service_unavailable', message: 'calm down', requestId: 'req-3' } },
      },
    ]);

    render(<ScenarioPanel />);
    await screen.findByTestId('scenario-panel');

    fireEvent.change(dataModeSelect(), { target: { value: 'live' } });

    const error = await screen.findByTestId('scenario-error');
    expect(error).toHaveTextContent('update failed');
    expect(error).toHaveAttribute('title', 'calm down');
    // Reverted: the select shows the last server-confirmed value again.
    expect(dataModeSelect().value).toBe('fake');
    expect(injectFailureSelect().value).toBe('none');
  });

  it('degrades to a disabled unavailable note when the initial GET fails — never a crash', async () => {
    stubRecordingFetch([
      {
        url: '/api/demo/scenario',
        status: 500,
        body: { error: { code: 'internal', message: 'boom', requestId: 'req-1' } },
      },
    ]);

    render(<ScenarioPanel />);

    expect(await screen.findByTestId('scenario-unavailable')).toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.queryByTestId('scenario-error')).not.toBeInTheDocument();
  });

  it('allows only one PATCH in flight: a second change during a slow round is ignored', async () => {
    const calls = stubRecordingFetch([
      fakeNone,
      {
        url: '/api/demo/scenario',
        method: 'PATCH',
        delayMs: 30,
        body: { dataMode: 'live', injectFailure: 'none' },
      },
    ]);

    render(<ScenarioPanel />);
    await screen.findByTestId('scenario-panel');

    fireEvent.change(dataModeSelect(), { target: { value: 'live' } });
    // The round is in flight (selects disabled); a slipped-in second change
    // must not start a concurrent PATCH.
    expect(dataModeSelect()).toBeDisabled();
    fireEvent.change(injectFailureSelect(), { target: { value: 'error' } });

    await waitFor(() => expect(dataModeSelect().value).toBe('live'));
    expect(calls.filter((call) => call.init?.method === 'PATCH')).toHaveLength(1);
    expect(injectFailureSelect().value).toBe('none');
  });

  it('never PATCHes on mount, including under StrictMode double-mount', async () => {
    const calls = stubRecordingFetch([fakeNone]);

    render(
      <StrictMode>
        <ScenarioPanel />
      </StrictMode>,
    );

    await screen.findByTestId('scenario-panel');
    await waitFor(() =>
      expect(calls.some((call) => call.init?.method === 'PATCH')).toBe(false),
    );
    // Both mounts' GETs are idempotent reads; exactly two rounds in dev.
    expect(calls.filter((call) => (call.init?.method ?? 'GET') === 'GET')).toHaveLength(2);
  });
});
