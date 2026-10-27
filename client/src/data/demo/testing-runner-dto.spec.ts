import { describe, expect, it } from 'vitest';
import { testingRunnerDtoFromJson } from './testing-runner-dto';

/**
 * TestingRunnerDTO spec — the fromJson whitelist: a full payload builds
 * the domain shape (run panel, package rows, timeline with subtests,
 * counted shape); every violation (missing panels, wrong types, an
 * unknown lifecycle state) throws so parseDto can upgrade it to an
 * InvalidResponseException.
 */

const fullPayload = {
  mode: 'boot-cached: the curated run executes once at process boot',
  run: {
    state: 'done',
    reason: 'the curated run exceeded nothing — this fixture run succeeded',
    command: "go test -json -count=1 -run '…' ./internal/domain/posts",
    startedAt: '2026-10-27T09:00:00Z',
    finishedAt: '2026-10-27T09:00:01Z',
    durationMs: 1000.5,
    ageSeconds: 61,
    exitCode: 0,
    totalPassed: 2,
    totalFailed: 0,
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
      package: 'guide/internal/data/db',
      outcome: 'pass',
      passed: 0,
      failed: 0,
      skipped: 1,
      durationMs: 5,
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
    provenance: 'every count above was parsed from the source tree',
  },
  reasoning: 'every row was parsed from the go tool events',
};

describe('testingRunnerDtoFromJson', () => {
  it('builds the domain payload from a valid wire body', () => {
    const dto = testingRunnerDtoFromJson(fullPayload);

    expect(dto.run.state).toBe('done');
    expect(dto.run.totalPassed).toBe(2);
    expect(dto.run.ageSeconds).toBe(61);
    expect(dto.packages).toHaveLength(2);
    expect(dto.packages[1]?.skipped).toBe(1);
    expect(dto.tests[1]?.depth).toBe(1);
    expect(dto.shape.testFunctions).toBe(640);
    expect(dto.shape.tableExamples[0]?.present).toBe(true);
    expect(dto.shape.tableExamples[1]?.present).toBe(false);
  });

  it('keeps optional fields absent-when-unset (a running run has no finish)', () => {
    const { reason: _r, startedAt: _s, finishedAt: _f, ...running } = fullPayload.run;
    const dto = testingRunnerDtoFromJson({
      ...fullPayload,
      run: { ...running, state: 'running' },
      tests: [],
    });

    expect(dto.run.state).toBe('running');
    expect(dto.run.startedAt).toBeUndefined();
    expect(dto.run.reason).toBeUndefined();
    expect(dto.tests).toEqual([]);
  });

  it('rejects an unknown run lifecycle state', () => {
    const broken = { ...fullPayload, run: { ...fullPayload.run, state: 'maybe' } };

    expect(() => testingRunnerDtoFromJson(broken)).toThrow(/running\|done\|unavailable/);
  });

  it('rejects a payload missing the package rows', () => {
    const { packages: _p, ...withoutPackages } = fullPayload;

    expect(() => testingRunnerDtoFromJson(withoutPackages)).toThrow();
  });

  it('rejects a test row whose depth is not a number', () => {
    const broken = {
      ...fullPayload,
      tests: [{ ...fullPayload.tests[0], depth: 'one' }],
    };

    expect(() => testingRunnerDtoFromJson(broken)).toThrow();
  });

  it('rejects a table example whose present is not a boolean', () => {
    const broken = {
      ...fullPayload,
      shape: {
        ...fullPayload.shape,
        tableExamples: [{ ...fullPayload.shape.tableExamples[0], present: 'yes' }],
      },
    };

    expect(() => testingRunnerDtoFromJson(broken)).toThrow();
  });
});
