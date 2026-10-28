import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { PerfBenchDemo } from './bench_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetPerfBenchDemoUseCase,
  PERF_BENCH_DEMO_USE_CASE,
} from '../../../domain/demo/get-perf-bench-demo-usecase';
import type { PerfBenchDemoData } from '../../../domain/demo/perf-bench';

/**
 * PerfBenchDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token. Behavior, not
 * implementation: the run panel states the exact command with its
 * benchtime/count/age; the compression curve renders level × size with
 * the ratio and BOTH count samples side by side (never averaged); the
 * encode and hot-path rows render their samples; the escape panel
 * renders the compiler verdicts verbatim; and failures degrade to the
 * shared error panel with recovery.
 */

const fixture: PerfBenchDemoData = {
  mode: 'boot-cached: the curated benchmarks execute ONCE at process boot',
  run: {
    state: 'done',
    command:
      "go test -bench=^BenchmarkPerf -benchtime=10x -count=2 -benchmem -run='^$' ./internal/domain/demos/",
    startedAt: '2026-10-28T06:00:00Z',
    finishedAt: '2026-10-28T06:00:04Z',
    durationMs: 4183.5,
    ageSeconds: 61,
    exitCode: 0,
    goos: 'linux',
    goarch: 'arm64',
    cpu: 'family bench CPU',
    pkg: 'guide/internal/domain/demos',
    benchtime: '10x',
    count: 2,
    failedBenchmarks: 0,
  },
  compression: [
    {
      level: 1,
      size: '4KB',
      rawBytes: 4231,
      compressedBytes: 1420,
      ratio: '3.0x',
      samples: [
        { iterations: 10, nsPerOp: 93000, bytesPerOp: 24576, allocsPerOp: 3, hasMemStats: true },
        { iterations: 10, nsPerOp: 101000, bytesPerOp: 24576, allocsPerOp: 3, hasMemStats: true },
      ],
    },
    {
      level: 9,
      size: '4KB',
      rawBytes: 4231,
      compressedBytes: 1190,
      ratio: '3.6x',
      samples: [
        { iterations: 10, nsPerOp: 240000, bytesPerOp: 24576, allocsPerOp: 3, hasMemStats: true },
        { iterations: 10, nsPerOp: 251000, bytesPerOp: 24576, allocsPerOp: 3, hasMemStats: true },
      ],
    },
  ],
  encoding: [
    {
      approach: 'buffered: json.Marshal',
      subtest: 'approach=marshal',
      note: 'one whole-document allocation handed to the caller',
      samples: [{ iterations: 10, nsPerOp: 41000, bytesPerOp: 65536, allocsPerOp: 2, hasMemStats: true }],
    },
    {
      approach: 'streaming: json.NewEncoder(w).Encode',
      subtest: 'approach=stream',
      note: 'the shape writeJSON uses on every response',
      samples: [{ iterations: 10, nsPerOp: 39000, bytesPerOp: 1024, allocsPerOp: 4, hasMemStats: true }],
    },
  ],
  hotPath: {
    benchmark: 'BenchmarkPerfInMemoryPostsList',
    note: "the fake mode's REAL serving path — the defensive-copy discipline, measured",
    samples: [{ iterations: 10, nsPerOp: 2100, bytesPerOp: 1224, allocsPerOp: 9, hasMemStats: true }],
  },
  escape: {
    command: 'go build -gcflags=-m ./internal/presentation/',
    state: 'done',
    verdicts: [
      { site: 'internal/presentation/posts.go:55', verdict: 'escapes to heap', line: 'internal/presentation/posts.go:55:20: append escapes to heap' },
      { site: 'internal/presentation/content.go:71', verdict: 'inlining call to', line: 'internal/presentation/content.go:71:21: inlining call to json.NewEncoder' },
    ],
  },
  checklist: [
    'Budgets — write the numbers down before you tune.',
    'Profile — capture pprof profiles from the real binary under load.',
  ],
  reasoning: 'every number above was parsed from the go tool’s own output',
};

function stubUseCase(execute: () => Promise<Result<PerfBenchDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    PERF_BENCH_DEMO_USE_CASE,
    { execute } as unknown as GetPerfBenchDemoUseCase,
  );
}

describe('PerfBenchDemo', () => {
  it('renders loading first, then the run, curve, encode, escape, and checklist panels', async () => {
    stubUseCase(async () => success(fixture));

    render(<PerfBenchDemo />);

    expect(screen.getByTestId('perf-bench-loading')).toBeInTheDocument();

    await waitForReady();
    const run = screen.getByTestId('perf-bench-run');
    expect(run).toHaveTextContent('go test -bench=^BenchmarkPerf');
    expect(run).toHaveTextContent('benchtime 10x · count 2');
    expect(run).toHaveTextContent('61s ago');

    const compression = screen.getByTestId('perf-bench-compression');
    expect(compression).toHaveTextContent('gzip 1 · 4KB');
    expect(compression).toHaveTextContent('3.0x');
    // BOTH count samples render, side by side — never averaged.
    expect(compression).toHaveTextContent('93.0µs');
    expect(compression).toHaveTextContent('101.0µs');
    expect(compression).toHaveTextContent('240.0µs');
    expect(compression).toHaveTextContent('3 allocs/op');

    const encoding = screen.getByTestId('perf-bench-encoding');
    expect(encoding).toHaveTextContent('buffered: json.Marshal');
    expect(encoding).toHaveTextContent('streaming: json.NewEncoder(w).Encode');
    expect(encoding).toHaveTextContent('BenchmarkPerfInMemoryPostsList');
    expect(encoding).toHaveTextContent('9 allocs/op');

    const escape = screen.getByTestId('perf-bench-escape');
    expect(escape).toHaveTextContent('go build -gcflags=-m ./internal/presentation/');
    expect(escape).toHaveTextContent('internal/presentation/posts.go:55');
    expect(escape).toHaveTextContent('escapes to heap');

    const checklist = screen.getByTestId('perf-bench-checklist');
    expect(checklist).toHaveTextContent('Budgets');

    expect(screen.getByTestId('perf-bench-reasoning')).toHaveTextContent('parsed from the go tool');
  });

  it('renders the running state honestly while the boot run is in flight', async () => {
    stubUseCase(async () =>
      success({
        ...fixture,
        run: {
          ...fixture.run,
          state: 'running',
          command: fixture.run.command,
          durationMs: 0,
          ageSeconds: 0,
        },
        compression: fixture.compression.map((cell) => ({ ...cell, samples: [] })),
      }),
    );

    render(<PerfBenchDemo />);
    await waitForReady();

    const run = screen.getByTestId('perf-bench-run');
    expect(run).toHaveTextContent('running');
    expect(run).toHaveTextContent('not yet finished');
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure(
      'The performance bench could not execute its curated run: the go toolchain is not on PATH.',
      { code: 'service_unavailable', status: 503, requestId: 'rid-perf-bench' },
    );
    stubUseCase(async () => failed(failure));

    render(<PerfBenchDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('perf bench');

    // Retry re-serves the cached run through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('perf-bench-ready')).toBeInTheDocument());
}
