import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { AppInfoDemoData } from './app-info';
import type { AppInfoRepository } from './app-info-repository';
import { GetAppInfoDemoUseCase } from './get-app-info-demo-usecase';

/**
 * GetAppInfoDemoUseCase spec — the use case is a thin pass-through seam,
 * so the contract to pin is exactly that: whatever the repository resolves
 * (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<AppInfoDemoData>): AppInfoRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: AppInfoDemoData = {
  module: { path: 'guide', version: '(devel)', goVersion: 'go1.27.3', dependencyCount: 0 },
  runtime: { numCPU: 8, goMaxProcs: 8 },
  environment: { port: 8100, env: 'dev', logLevel: 'INFO' },
  buildSettings: [{ key: 'GOARCH', value: 'arm64' }],
};

describe('GetAppInfoDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetAppInfoDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetAppInfoDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
