/**
 * Domain models for the performance topic's bench demo (demo block id
 * `perf:bench`) — the client-side shape of
 * `GET /api/demo/perf/bench`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload is RUN-DERIVED: the server executed this repo's own
 * curated benchmarks ONCE at process boot (single-flight, bounded by a
 * hard process-group-killing timeout) and parsed the go tool's output,
 * so RUN is the provenance panel (the exact command, benchtime, count,
 * wall-clock timing, environment headers, age), COMPRESSION is the
 * measured gzip curve (levels × sizes, each cell carrying the
 * deterministic document's raw/compressed sizes plus every parsed
 * count sample — side by side, never averaged), ENCODING is the
 * buffered-marshal versus streaming-encode A/B, HOTPATH is the fake
 * mode's real serving path with its allocation tax, ESCAPE is the
 * compiler's own -gcflags=-m verdicts served verbatim, and CHECKLIST
 * is the pre-production pass as data. No number on the wire was
 * invented; a run that could not happen degrades to the typed 503.
 */

/** The run's provenance panel: state, command, configuration, age. */
export interface PerfRunPanel {
  state: string;
  reason?: string;
  command: string;
  startedAt?: string;
  finishedAt?: string;
  durationMs: number;
  ageSeconds: number;
  exitCode: number;
  goos?: string;
  goarch?: string;
  cpu?: string;
  pkg?: string;
  benchtime: string;
  count: number;
  failedBenchmarks: number;
}

/** ONE count-run of one benchmark — the tool's metric pairs, as parsed. */
export interface PerfSample {
  iterations: number;
  nsPerOp: number;
  bytesPerOp: number;
  allocsPerOp: number;
  hasMemStats: boolean;
}

/** One cell of the compression curve: gzip level × payload size. */
export interface PerfCompressionRow {
  level: number;
  size: string;
  rawBytes: number;
  compressedBytes: number;
  ratio: string;
  samples: PerfSample[];
}

/** One side of the encode comparison (marshal vs streaming). */
export interface PerfEncodeRow {
  approach: string;
  subtest: string;
  note: string;
  samples: PerfSample[];
}

/** The allocation-focused benchmark of the fake mode's serving path. */
export interface PerfHotPathRow {
  benchmark: string;
  note: string;
  samples: PerfSample[];
}

/** One compiler escape-analysis verdict, served verbatim. */
export interface PerfEscapeVerdict {
  site: string;
  verdict: string;
  line: string;
}

/** The escape-analysis panel: command, state, curated verdicts. */
export interface PerfEscapePanel {
  command: string;
  state: string;
  reason?: string;
  verdicts: PerfEscapeVerdict[];
}

/** The whole perf bench payload. */
export interface PerfBenchDemoData {
  mode: string;
  run: PerfRunPanel;
  compression: PerfCompressionRow[];
  encoding: PerfEncodeRow[];
  hotPath: PerfHotPathRow;
  escape: PerfEscapePanel;
  checklist: string[];
  reasoning: string;
}
