import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { AppInfoApi } from './app-info-api';
import { AppInfoRepositoryImpl } from './app-info-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * AppInfoRepositoryImpl spec — fixture-driven (happy + failure): the happy
 * payload round-trips into the domain shape; network / envelope /
 * malformed-payload failures all resolve as Failed Results, never throws.
 */

/** The real builder output shape (observed: stdlib-only module, (devel)). */
const demoPayload = {
  module: { path: 'guide', version: '(devel)', goVersion: 'go1.27.3', dependencyCount: 0 },
  runtime: { numCPU: 8, goMaxProcs: 8 },
  environment: { port: 8100, env: 'dev', logLevel: 'INFO' },
  buildSettings: [
    { key: '-buildmode', value: 'exe' },
    { key: 'CGO_CFLAGS', value: '' },
    { key: 'GOARCH', value: 'arm64' },
    { key: 'GOOS', value: 'linux' },
  ],
};

function makeRepository(): AppInfoRepositoryImpl {
  return new AppInfoRepositoryImpl(new AppInfoApi(createAppConfig()));
}

describe('AppInfoRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/gohttp/app-info', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.module.path).toBe('guide');
      expect(result.value.module.dependencyCount).toBe(0);
      expect(result.value.runtime.goMaxProcs).toBe(8);
      expect(result.value.environment.logLevel).toBe('INFO');
      // An unset build flag's empty value is data and must survive.
      expect(result.value.buildSettings[1]?.value).toBe('');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });

  it('guards an error envelope into a Failed Result with its diagnostics', async () => {
    stubApiFetch([
      {
        url: '/api/demo/gohttp/app-info',
        status: 503,
        body: { error: { code: 'service_unavailable', message: 'calm down', requestId: 'req-4' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('calm down', {
          code: 'service_unavailable',
          status: 503,
          requestId: 'req-4',
        }),
      ),
    );
  });

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // Missing every section — the DTO validator must reject it.
      { url: '/api/demo/gohttp/app-info', body: { module: 'guide' } },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('The server response did not match the expected shape.', {
          code: 'invalid_response',
          status: 200,
        }),
      ),
    );
  });

  it('rejects a non-numeric port instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/gohttp/app-info',
        body: { ...demoPayload, environment: { ...demoPayload.environment, port: '8100' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });
});
