import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { PerfBenchApi } from './perf-bench-api';
import { PerfBenchRepositoryImpl } from './perf-bench-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * PerfBenchRepositoryImpl spec — fixture-driven (happy + failure): the
 * bench payload (run panel, compression samples, escape verdicts)
 * round-trips; malformed payloads and transport failures resolve as
 * Failed Results, never throws.
 */

const demoPayload = {
  mode: 'boot-cached: the curated benchmarks execute ONCE at process boot',
  run: {
    state: 'done',
    command: "go test -bench=^BenchmarkPerf -benchtime=10x -count=2 -benchmem -run='^$' ./internal/domain/demos/",
    durationMs: 4183.5,
    ageSeconds: 12,
    exitCode: 0,
    benchtime: '10x',
    count: 2,
    failedBenchmarks: 0,
  },
  compression: [
    {
      level: 6,
      size: '64KB',
      rawBytes: 65536,
      compressedBytes: 8192,
      ratio: '8.0x',
      samples: [
        { iterations: 10, nsPerOp: 93000, bytesPerOp: 24576, allocsPerOp: 3, hasMemStats: true },
        { iterations: 10, nsPerOp: 101000, bytesPerOp: 24576, allocsPerOp: 3, hasMemStats: true },
      ],
    },
  ],
  encoding: [
    {
      approach: 'buffered: json.Marshal',
      subtest: 'approach=marshal',
      note: 'one whole-document allocation',
      samples: [{ iterations: 10, nsPerOp: 41000, bytesPerOp: 65536, allocsPerOp: 2, hasMemStats: true }],
    },
  ],
  hotPath: {
    benchmark: 'BenchmarkPerfInMemoryPostsList',
    note: 'the defensive copies, measured',
    samples: [{ iterations: 10, nsPerOp: 2100, bytesPerOp: 1224, allocsPerOp: 9, hasMemStats: true }],
  },
  escape: {
    command: 'go build -gcflags=-m ./internal/presentation/',
    state: 'done',
    verdicts: [
      { site: 'internal/presentation/posts.go:55', verdict: 'escapes to heap', line: 'internal/presentation/posts.go:55:20: append escapes to heap' },
    ],
  },
  checklist: ['Budgets — write the numbers down before you tune.'],
  reasoning: 'parsed from the go tool’s own output, config attached',
};

function makeRepository(): PerfBenchRepositoryImpl {
  return new PerfBenchRepositoryImpl(new PerfBenchApi(createAppConfig()));
}

describe('PerfBenchRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/perf/bench', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.run.benchtime).toBe('10x');
      expect(result.value.compression[0]?.samples).toHaveLength(2);
      expect(result.value.escape.verdicts[0]?.verdict).toBe('escapes to heap');
    }
  });

  it('guards a malformed payload into a Failed Result', async () => {
    stubApiFetch([{ url: '/api/demo/perf/bench', body: { mode: 'half a payload' } }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
    }
  });
});
