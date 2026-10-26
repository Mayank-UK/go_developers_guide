/**
 * Domain models for the goroutines-and-channels topic's race-detector
 * demo — the client-side shape of `GET /api/demo/conc/race`. Field names
 * match the server's camelCase wire contract 1:1.
 */

/** The pinned racy exhibit: its code, the toolchain's verdict, the fixes. */
export interface RaceExhibit {
  /** The unsynchronized counter, pinned as data (never executed server-side). */
  code: string;
  /** The command whose output the verdict was captured from. */
  command: string;
  /** The race detector's own words, verbatim from a documented run. */
  verdict: string;
  /** What the detector checks and why the exhibit trips it. */
  explanation: string;
  /** The two synchronizations that fix it (run live beside the exhibit). */
  fix: string;
}

/** One live, race-free total: strategy, shape, and exact arithmetic. */
export interface SynchronizedRun {
  strategy: string;
  goroutines: number;
  incrementsEach: number;
  expected: number;
  observed: number;
  /** observed === expected — the whole point of synchronizing. */
  exact: boolean;
}

/** One live leaked-vs-done case from the hygiene beat. */
export interface HygieneRow {
  case: string;
  readerPresent: boolean;
  outcome: string;
}

export interface RaceDetectorDemoData {
  race: RaceExhibit;
  mutex: SynchronizedRun;
  channel: SynchronizedRun;
  hygiene: HygieneRow[];
}
