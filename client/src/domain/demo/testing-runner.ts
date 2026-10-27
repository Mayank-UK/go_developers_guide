/**
 * Domain models for the testing topic's runner demo (demo block id
 * `testing:runner`) — the client-side shape of
 * `GET /api/demo/testing/runner`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload's spine is a REAL, bounded `go test -json` run over a
 * curated set of the repo's own test packages, executed once at server
 * boot and served with its wall-clock age: the RUN panel carries the
 * lifecycle (running / done / unavailable), the exact command, and the
 * totals derived from parsed events. PACKAGES rows carry per-package
 * outcomes and counts; TESTS rows are the execution timeline, subtests
 * depth-annotated by their slashed names. SHAPE is the suite counted
 * from the source tree at request time (files, packages, functions,
 * t.Parallel calls) plus the curated table-test examples located by
 * symbol with presence re-verified — no number typed by hand.
 */

/** The run panel: lifecycle, command, timing, exit code, parsed totals. */
export interface TestingRunPanel {
  state: 'running' | 'done' | 'unavailable';
  reason?: string;
  command: string;
  startedAt?: string;
  finishedAt?: string;
  durationMs: number;
  ageSeconds: number;
  exitCode: number;
  totalPassed: number;
  totalFailed: number;
  totalSkipped: number;
}

/** One package's parsed outcome from the go tool's own events. */
export interface TestingPackageRow {
  package: string;
  outcome: string;
  passed: number;
  failed: number;
  skipped: number;
  durationMs: number;
}

/** One test (or subtest — depth says which) on the timeline. */
export interface TestingTestRow {
  package: string;
  test: string;
  status: string;
  depth: number;
  durationMs: number;
  output?: string;
}

/** One curated table-test example, located by symbol. */
export interface TestingTableExampleRow {
  package: string;
  file: string;
  symbol: string;
  note: string;
  present: boolean;
}

/** The suite's shape, counted live from the source tree. */
export interface TestingSuiteShape {
  testFiles: number;
  testPackages: number;
  testFunctions: number;
  parallelCalls: number;
  tableExamples: TestingTableExampleRow[];
  provenance: string;
}

/** The whole runner payload. */
export interface TestingRunnerDemoData {
  mode: string;
  run: TestingRunPanel;
  packages: TestingPackageRow[];
  tests: TestingTestRow[];
  shape: TestingSuiteShape;
  reasoning: string;
}
