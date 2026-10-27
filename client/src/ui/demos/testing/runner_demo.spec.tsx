import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { TestingRunnerDemo } from './runner_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetTestingRunnerDemoUseCase,
  TESTING_RUNNER_DEMO_USE_CASE,
} from '../../../domain/demo/get-testing-runner-demo-usecase';
import type { TestingRunnerDemoData } from '../../../domain/demo/testing-runner';

/**
 * TestingRunnerDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token.
 * Behavior, not implementation: the run panel states the mode and the
 * parsed totals; the package rows carry outcomes and counts; the
 * timeline indents subtests by depth and shows failure excerpts only on
 * red rows; the shape panel counts from the tree and verifies the
 * table-test trio; and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: TestingRunnerDemoData = {
  mode: 'boot-cached: the curated run executes once at process boot in the background',
  run: {
    state: 'done',
    command: "go test -json -count=1 -run '…' ./internal/domain/posts ./internal/presentation",
    startedAt: '2026-10-27T09:00:00Z',
    finishedAt: '2026-10-27T09:00:01Z',
    durationMs: 1010,
    ageSeconds: 61,
    exitCode: 0,
    totalPassed: 2,
    totalFailed: 1,
    totalSkipped: 1,
  },
  packages: [
    {
      package: 'guide/internal/domain/posts',
      outcome: 'pass',
      passed: 1,
      failed: 0,
      skipped: 0,
      durationMs: 7.2,
    },
    {
      package: 'guide/internal/presentation',
      outcome: 'fail',
      passed: 1,
      failed: 1,
      skipped: 0,
      durationMs: 310,
    },
  ],
  tests: [
    {
      package: 'guide/internal/domain/posts',
      test: 'TestDraftValidateCollectsEveryIssue',
      status: 'pass',
      depth: 0,
      durationMs: 4,
    },
    {
      package: 'guide/internal/presentation',
      test: 'TestPostsCreateRefusesBodiesAtTheBoundary',
      status: 'fail',
      depth: 0,
      durationMs: 12,
      output: '    posts_test.go:199: body does not carry "field"',
    },
    {
      package: 'guide/internal/presentation',
      test: 'TestPostsCreateRefusesBodiesAtTheBoundary/null_body',
      status: 'pass',
      depth: 1,
      durationMs: 1,
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
      {
        package: 'guide/internal/domain/obs',
        file: 'internal/domain/obs/tracecontext_test.go',
        symbol: 'TestParseTraceparentRejectsMalformedHeaders',
        note: 'the spec edge table',
        present: false,
      },
    ],
    provenance: 'every count above was parsed from the source tree beside the process',
  },
  reasoning: 'every row was parsed from the go tool events or counted from the tree',
};

function stubUseCase(execute: () => Promise<Result<TestingRunnerDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    TESTING_RUNNER_DEMO_USE_CASE,
    { execute } as unknown as GetTestingRunnerDemoUseCase,
  );
}

describe('TestingRunnerDemo', () => {
  it('renders loading first, then the run panel, packages, timeline, and shape', async () => {
    stubUseCase(async () => success(fixture));

    render(<TestingRunnerDemo />);

    expect(screen.getByTestId('testing-runner-loading')).toBeInTheDocument();

    await waitForReady();
    const run = screen.getByTestId('testing-runner-run');
    expect(run).toHaveTextContent('boot-cached');
    expect(run).toHaveTextContent('2 passed');
    expect(run).toHaveTextContent('1 failed');
    expect(run).toHaveTextContent("go test -json -count=1 -run");

    const packages = screen.getByTestId('testing-runner-packages');
    expect(packages).toHaveTextContent('guide/internal/domain/posts');
    expect(packages).toHaveTextContent('guide/internal/presentation');

    const timeline = screen.getByTestId('testing-runner-timeline');
    expect(timeline).toHaveTextContent('TestDraftValidateCollectsEveryIssue');
    expect(timeline).toHaveTextContent('TestPostsCreateRefusesBodiesAtTheBoundary/null_body');
    // The failed row carries its excerpt; passing rows carry none.
    expect(timeline).toHaveTextContent('posts_test.go:199');

    const shape = screen.getByTestId('testing-runner-shape');
    expect(shape).toHaveTextContent('24 packages');
    expect(shape).toHaveTextContent('640 functions');
    expect(shape).toHaveTextContent('120 t.Parallel calls');
    expect(shape).toHaveTextContent('✓ TestLoadConfig');
    expect(shape).toHaveTextContent('✗ TestParseTraceparentRejectsMalformedHeaders');
    expect(shape).toHaveTextContent('parsed from the source tree');

    expect(screen.getByTestId('testing-runner-reasoning')).toHaveTextContent('go tool events');
  });

  it('renders the honest empty timeline while the boot run is still in flight', async () => {
    stubUseCase(async () =>
      success({
        ...fixture,
        run: { ...fixture.run, state: 'running', totalPassed: 0, totalFailed: 0, totalSkipped: 0 },
        packages: [],
        tests: [],
      }),
    );

    render(<TestingRunnerDemo />);
    await waitForReady();

    expect(screen.getByTestId('testing-runner-timeline')).toHaveTextContent('no invented rows');
    expect(screen.getByTestId('testing-runner-run')).toHaveTextContent('running');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure('The testing runner could not be reached.', {
      code: 'network_error',
      status: 0,
      requestId: 'rid-runner-down',
    });
    stubUseCase(async () => failed(failure));

    render(<TestingRunnerDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('testing-runner-ready')).toBeInTheDocument());
}
