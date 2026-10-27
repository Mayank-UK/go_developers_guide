import type {
  TestingPackageRow,
  TestingRunPanel,
  TestingRunnerDemoData,
  TestingSuiteShape,
  TestingTableExampleRow,
  TestingTestRow,
} from '../../domain/demo/testing-runner';
import {
  isRecord,
  optionalString,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * TestingRunnerDTO — the wire/transport shape of the runner payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException. Optional wire fields
 * (reason, startedAt, finishedAt, a failed test's output) map to
 * omitted-when-absent — never null.
 */

export type TestingRunnerDemoDto = TestingRunnerDemoData;

function runPanelFromJson(body: Record<string, unknown>): TestingRunPanel {
  const run = requiredRecord(body, 'run');
  const state = requiredString(run, 'state');
  if (state !== 'running' && state !== 'done' && state !== 'unavailable') {
    throw new Error(`run.state must be running|done|unavailable, got ${state}`);
  }
  return {
    state,
    reason: optionalString(run, 'reason'),
    command: requiredString(run, 'command'),
    startedAt: optionalString(run, 'startedAt'),
    finishedAt: optionalString(run, 'finishedAt'),
    durationMs: requiredNumber(run, 'durationMs'),
    ageSeconds: requiredNumber(run, 'ageSeconds'),
    exitCode: requiredNumber(run, 'exitCode'),
    totalPassed: requiredNumber(run, 'totalPassed'),
    totalFailed: requiredNumber(run, 'totalFailed'),
    totalSkipped: requiredNumber(run, 'totalSkipped'),
  };
}

function packageRowFromJson(value: unknown): TestingPackageRow {
  if (!isRecord(value)) throw new Error('package row must be an object');
  return {
    package: requiredString(value, 'package'),
    outcome: requiredString(value, 'outcome'),
    passed: requiredNumber(value, 'passed'),
    failed: requiredNumber(value, 'failed'),
    skipped: requiredNumber(value, 'skipped'),
    durationMs: requiredNumber(value, 'durationMs'),
  };
}

function testRowFromJson(value: unknown): TestingTestRow {
  if (!isRecord(value)) throw new Error('test row must be an object');
  return {
    package: requiredString(value, 'package'),
    test: requiredString(value, 'test'),
    status: requiredString(value, 'status'),
    depth: requiredNumber(value, 'depth'),
    durationMs: requiredNumber(value, 'durationMs'),
    output: optionalString(value, 'output'),
  };
}

function tableExampleFromJson(value: unknown): TestingTableExampleRow {
  if (!isRecord(value)) throw new Error('table example must be an object');
  return {
    package: requiredString(value, 'package'),
    file: requiredString(value, 'file'),
    symbol: requiredString(value, 'symbol'),
    note: requiredString(value, 'note'),
    present: requiredBoolean(value, 'present'),
  };
}

function shapeFromJson(body: Record<string, unknown>): TestingSuiteShape {
  const shape = requiredRecord(body, 'shape');
  return {
    testFiles: requiredNumber(shape, 'testFiles'),
    testPackages: requiredNumber(shape, 'testPackages'),
    testFunctions: requiredNumber(shape, 'testFunctions'),
    parallelCalls: requiredNumber(shape, 'parallelCalls'),
    tableExamples: requiredArray(shape, 'tableExamples').map(tableExampleFromJson),
    provenance: requiredText(shape, 'provenance'),
  };
}

/** Deserialize + validate the GET /api/demo/testing/runner payload. */
export function testingRunnerDtoFromJson(body: unknown): TestingRunnerDemoDto {
  if (!isRecord(body)) throw new Error('testing runner payload must be an object');
  return {
    // Present-but-empty is real data: the mode line names the shipped mode.
    mode: requiredText(body, 'mode'),
    run: runPanelFromJson(body),
    packages: requiredArray(body, 'packages').map(packageRowFromJson),
    tests: requiredArray(body, 'tests').map(testRowFromJson),
    shape: shapeFromJson(body),
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toTestingRunnerDemo(dto: TestingRunnerDemoDto): TestingRunnerDemoData {
  return dto;
}
