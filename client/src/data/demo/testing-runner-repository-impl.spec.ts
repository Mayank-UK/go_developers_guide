import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { TestingRunnerApi } from './testing-runner-api';
import { TestingRunnerRepositoryImpl } from './testing-runner-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * TestingRunnerRepositoryImpl spec — fixture-driven (happy + failure):
 * the runner payload (run panel, packages, timeline, shape) round-trips;
 * malformed payloads and transport failures resolve as Failed Results,
 * never throws; and the server's typed 503 (toolchain hiccup) guards
 * into a ServerFailure carrying the envelope's code.
 */

const demoPayload = {
  mode: 'boot-cached: the curated run executes once at process boot',
  run: {
    state: 'done',
    command: "go test -json -count=1 -run '…' ./internal/domain/posts",
    startedAt: '2026-10-27T09:00:00Z',
    finishedAt: '2026-10-27T09:00:01Z',
    durationMs: 1000.5,
    ageSeconds: 42,
    exitCode: 0,
    totalPassed: 5,
    totalFailed: 0,
    totalSkipped: 0,
  },
  packages: [
    {
      package: 'guide/internal/presentation',
      outcome: 'pass',
      passed: 2,
      failed: 0,
      skipped: 0,
      durationMs: 310,
    },
  ],
  tests: [
    {
      package: 'guide/internal/presentation',
      test: 'TestPostsListServesTheSeedPage',
      status: 'pass',
      depth: 0,
      durationMs: 12,
    },
  ],
  shape: {
    testFiles: 96,
    testPackages: 24,
    testFunctions: 640,
    parallelCalls: 120,
    tableExamples: [
      {
        package: 'guide/internal/core',
        file: 'internal/core/config_parsing_test.go',
        symbol: 'TestLoadConfig',
        note: 'the parallel exemplar',
        present: true,
      },
    ],
    provenance: 'every count above was parsed from the source tree',
  },
  reasoning: 'every row was parsed from the go tool events',
};

function makeRepository(): TestingRunnerRepositoryImpl {
  return new TestingRunnerRepositoryImpl(new TestingRunnerApi(createAppConfig()));
}

describe('TestingRunnerRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/testing/runner', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.run.totalPassed).toBe(5);
      expect(result.value.packages[0]?.package).toBe('guide/internal/presentation');
      expect(result.value.shape.tableExamples[0]?.symbol).toBe('TestLoadConfig');
    }
  });

  it('rejects a payload whose shape panel is missing', async () => {
    stubApiFetch([
      { url: '/api/demo/testing/runner', body: { ...demoPayload, shape: undefined } },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards the typed 503 (toolchain hiccup) into a Failed Result', async () => {
    stubApiFetch([
      {
        url: '/api/demo/testing/runner',
        status: 503,
        body: {
          error: {
            code: 'service_unavailable',
            message: 'The testing runner could not execute its curated run.',
            requestId: 'rid-runner-down',
          },
        },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('service_unavailable');
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
});
