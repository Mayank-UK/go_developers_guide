import { afterEach, describe, expect, it, vi } from 'vitest';
import { NetworkFailure, ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { ScenarioApi } from './scenario-api';
import { ScenarioRepositoryImpl } from './scenario-repository-impl';
import type { ScenarioState } from '../../domain/scenario/scenario';

afterEach(() => {
  vi.unstubAllGlobals();
});

/** Minimal Response stand-in: the API layer only reads ok/status/json(). */
function jsonResponse(body: unknown, status: number) {
  return { ok: status >= 200 && status < 300, status, json: async () => body };
}

function makeRepository(): ScenarioRepositoryImpl {
  return new ScenarioRepositoryImpl(new ScenarioApi(createAppConfig()));
}

const snapshot: ScenarioState = { dataMode: 'fake', injectFailure: 'none' };

describe('ScenarioRepositoryImpl', () => {
  it('returns a Success with the domain model for a snapshot', async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({ dataMode: 'fake', injectFailure: 'none' }, 200),
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await makeRepository().getScenario();

    expect(result).toEqual({ ok: true, value: snapshot });
    expect(fetchMock).toHaveBeenCalledWith('/api/demo/scenario', expect.anything());
  });

  it('sends a PATCH body carrying ONLY the patched key', async () => {
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        jsonResponse({ dataMode: 'fake', injectFailure: 'latency' }, 200),
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await makeRepository().patchScenario({ injectFailure: 'latency' });

    expect(result).toEqual({
      ok: true,
      value: { dataMode: 'fake', injectFailure: 'latency' },
    });
    // Exactly one PATCH round, against the scenario path, with the partial body.
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe('/api/demo/scenario');
    expect(init?.method).toBe('PATCH');
    expect(JSON.parse(String(init?.body))).toEqual({ injectFailure: 'latency' });
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('Failed to fetch');
      }),
    );

    const result = await makeRepository().getScenario();

    expect(result).toEqual(
      failed(new NetworkFailure('Could not reach the server.')),
    );
  });

  it('guards the 422 envelope for an invalid value into a Failed Result with diagnostics', async () => {
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        jsonResponse(
          {
            error: {
              code: 'validation_failed',
              message: 'Invalid scenario patch: unknown failure injection.',
              requestId: 'req-422',
              details: { injectFailure: 'bogus', expected: ['none', 'error', 'latency'] },
            },
          },
          422,
        ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await makeRepository().patchScenario({ injectFailure: 'bogus' as never });

    expect(result).toEqual(
      failed(
        new ServerFailure('Invalid scenario patch: unknown failure injection.', {
          code: 'validation_failed',
          status: 422,
          requestId: 'req-422',
        }),
      ),
    );
    // The PATCH round was still sent (the server, not the client, rejects it).
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe('PATCH');
  });

  it('guards a malformed snapshot into a Failed Result (invalid_response)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ dataMode: 'fake', injectFailure: 'bogus' }, 200)),
    );

    const result = await makeRepository().getScenario();

    expect(result).toEqual(
      failed(
        new ServerFailure('The server response did not match the expected shape.', {
          code: 'invalid_response',
          status: 200,
        }),
      ),
    );
  });
});
