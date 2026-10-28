import { describe, expect, it } from 'vitest';
import { perfBenchDtoFromJson } from './perf-bench-dto';

/**
 * PerfBenchDTO spec — the fromJson whitelist: a full payload builds
 * the domain shape (the run panel with its command and configuration,
 * the compression curve with side-by-side count samples, the encode
 * A/B, the hot path, the escape verdicts, the checklist); every
 * violation (missing panels, wrong types, a sample without its
 * metrics) throws so parseDto can upgrade it to an
 * InvalidResponseException.
 */

const fullPayload = {
  mode: 'boot-cached: the curated benchmarks execute ONCE at process boot',
  run: {
    state: 'done',
    command:
      "go test -bench=^BenchmarkPerf -benchtime=10x -count=2 -benchmem -run='^$' ./internal/domain/demos/",
    startedAt: '2026-10-28T06:00:00Z',
    finishedAt: '2026-10-28T06:00:04Z',
    durationMs: 4183.5,
    ageSeconds: 61.2,
    exitCode: 0,
    goos: 'linux',
    goarch: 'arm64',
    cpu: '13th Gen Intel(R) Core(TM) i7-13700K',
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
      size: '512KB',
      rawBytes: 524288,
      compressedBytes: 40211,
      ratio: '13.0x',
      samples: [{ iterations: 10, nsPerOp: 2400000, bytesPerOp: 24576, allocsPerOp: 3, hasMemStats: true }],
    },
  ],
  encoding: [
    {
      approach: 'buffered: json.Marshal',
      subtest: 'approach=marshal',
      note: 'serializes into one whole-document allocation',
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
    note: "the fake mode's REAL serving path",
    samples: [
      { iterations: 10, nsPerOp: 2100, bytesPerOp: 1224, allocsPerOp: 9, hasMemStats: true },
      { iterations: 10, nsPerOp: 2300, bytesPerOp: 1224, allocsPerOp: 9, hasMemStats: true },
    ],
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
    'Watch — /metrics is the running verdict.',
    'Bench — A/B every optimization with benchstat.',
    'Load — rehearse production traffic before production.',
  ],
  reasoning: 'every number above was parsed from the go tool’s own output',
};

describe('perfBenchDtoFromJson', () => {
  it('builds the domain payload from a valid wire body', () => {
    const dto = perfBenchDtoFromJson(fullPayload);

    expect(dto.run.command).toContain('-benchtime=10x');
    expect(dto.run.benchtime).toBe('10x');
    expect(dto.run.count).toBe(2);
    expect(dto.run.ageSeconds).toBeGreaterThan(60);
    expect(dto.compression).toHaveLength(2);
    expect(dto.compression[0]?.samples).toHaveLength(2);
    expect(dto.compression[0]?.samples[1]?.nsPerOp).toBe(101000);
    expect(dto.compression[1]?.ratio).toBe('13.0x');
    expect(dto.encoding[1]?.approach).toContain('streaming');
    expect(dto.hotPath.benchmark).toBe('BenchmarkPerfInMemoryPostsList');
    expect(dto.escape.verdicts[0]?.site).toBe('internal/presentation/posts.go:55');
    expect(dto.checklist).toHaveLength(5);
  });

  it('keeps optional panels honest: absent env headers stay undefined', () => {
    const { goos, cpu, ...withoutEnv } = fullPayload.run;
    const dto = perfBenchDtoFromJson({ ...fullPayload, run: withoutEnv });

    expect(dto.run.goos).toBeUndefined();
    expect(dto.run.cpu).toBeUndefined();
    expect(dto.run.state).toBe('done');
  });

  it('throws when the body is not an object', () => {
    expect(() => perfBenchDtoFromJson(null)).toThrow();
    expect(() => perfBenchDtoFromJson('perf bench')).toThrow();
  });

  it('throws when a top-level panel is missing', () => {
    const { run, ...withoutRun } = fullPayload;
    expect(() => perfBenchDtoFromJson(withoutRun)).toThrow(/run/);

    const { escape, ...withoutEscape } = fullPayload;
    expect(() => perfBenchDtoFromJson(withoutEscape)).toThrow(/escape/);

    const { hotPath, ...withoutHotPath } = fullPayload;
    expect(() => perfBenchDtoFromJson(withoutHotPath)).toThrow(/hotPath/);
  });

  it('throws when a sample carries the wrong types', () => {
    expect(() =>
      perfBenchDtoFromJson({
        ...fullPayload,
        compression: [
          {
            ...fullPayload.compression[0],
            samples: [{ iterations: 'ten', nsPerOp: 1, bytesPerOp: 1, allocsPerOp: 1, hasMemStats: true }],
          },
        ],
      }),
    ).toThrow(/iterations/);

    expect(() =>
      perfBenchDtoFromJson({
        ...fullPayload,
        hotPath: { ...fullPayload.hotPath, samples: 'none' },
      }),
    ).toThrow(/samples/);
  });

  it('throws when the checklist carries a non-string item', () => {
    expect(() =>
      perfBenchDtoFromJson({ ...fullPayload, checklist: ['Budgets — write them down.', 42] }),
    ).toThrow(/checklist/);
  });
});
