import { afterEach, describe, expect, it, vi } from 'vitest';
import { NetworkFailure, ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { HealthApi } from './health-api';
import { HealthRepositoryImpl } from './health-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/** Minimal Response stand-in: the API layer only reads ok/status/json(). */
function jsonResponse(body: unknown, status: number) {
  return { ok: status >= 200 && status < 300, status, json: async () => body };
}

function makeRepository(): HealthRepositoryImpl {
  return new HealthRepositoryImpl(new HealthApi(createAppConfig()));
}

describe('HealthRepositoryImpl', () => {
  it('returns a Success with the domain model for a healthy payload', async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({ status: 'ok', env: 'dev' }, 200),
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await makeRepository().getHealth();

    expect(result).toEqual({ ok: true, value: { status: 'ok', env: 'dev' } });
    expect(fetchMock).toHaveBeenCalledWith('/api/health', expect.anything());
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('Failed to fetch');
      }),
    );

    const result = await makeRepository().getHealth();

    expect(result).toEqual(
      failed(new NetworkFailure('Could not reach the server.')),
    );
  });

  it('guards a non-2xx response into a Failed Result that keeps the status', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({}, 503)));

    const result = await makeRepository().getHealth();

    expect(result).toEqual(
      failed(
        new ServerFailure('Request failed with status 503.', {
          code: 'unknown_error',
          status: 503,
        }),
      ),
    );
  });

  it('guards an error-envelope response into a Failed Result carrying its diagnostics', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(
          { error: { code: 'overloaded', message: 'calm down', requestId: 'req-7' } },
          503,
        ),
      ),
    );

    const result = await makeRepository().getHealth();

    expect(result).toEqual(
      failed(
        new ServerFailure('calm down', {
          code: 'overloaded',
          status: 503,
          requestId: 'req-7',
        }),
      ),
    );
  });

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ status: 'ok' }, 200)));

    const result = await makeRepository().getHealth();

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
